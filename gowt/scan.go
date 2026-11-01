package gowt

import (
	"fmt"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	"github.com/mandelsoft/vfs/pkg/vfs"
)

// GoFileInfo holds the information extracted from a .go source file.
type GoFileInfo struct {
	FS             vfs.FileSystem    // filesystem used to read this file and its neighbours
	Package        string
	Dir            string
	Imports        map[string]string // local alias → import path
	Instantiations []InstantiationSpec
}

// ExtraTypeArg is an additional type argument beyond the base type in a directive.
type ExtraTypeArg struct {
	Ptr      bool   // leading *
	PkgAlias string // package alias or full import path
	TypeName string // unqualified type name
}

// InstantiationSpec is one parsed "// type X wrapping [pkg.]W[T via Ctor, T2, T3]" directive.
type InstantiationSpec struct {
	TypeName      string            // X  — the generated concrete type name
	TmplPkg       string            // package alias/path for the template (empty = current package)
	TmplName      string            // W  — the template type name in the _gowt.go file
	BasePtr       bool              // true when the base type is written as *Pkg.Type
	BasePkgAlias  string            // local alias of the base type's package (e.g. "common")
	BaseTypeName  string            // unqualified base type name (e.g. "Environment")
	CtorPkgAlias  string            // local alias of the constructor's package (e.g. "common")
	CtorFuncName  string            // unqualified constructor function name (e.g. "New")
	ExtraTypeArgs []ExtraTypeArg    // additional type arguments after the base+via
	UsingPkgs     map[string]string // using clause mappings: "" → PACKAGE. default, "name" → gowt.<name>
	Renames       map[string]string // optional explicit method renames: old → new
}

// scanGoFile parses path using fs and returns its package, imports and gowt directives.
func scanGoFile(fs vfs.FileSystem, path string) (*GoFileInfo, error) {
	src, err := vfs.ReadFile(fs, path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	info := &GoFileInfo{
		FS:      fs,
		Package: f.Name.Name,
		Dir:     filepath.Dir(path),
		Imports: make(map[string]string),
	}

	for _, imp := range f.Imports {
		ipath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			parts := strings.Split(ipath, "/")
			alias = parts[len(parts)-1]
		}
		info.Imports[alias] = ipath
	}

	for _, cg := range f.Comments {
		for _, c := range cg.List {
			inst, ok, err := parseInstComment(c.Text)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", fset.Position(c.Slash), err)
			}
			if ok {
				info.Instantiations = append(info.Instantiations, inst)
			}
		}
	}

	return info, nil
}

// parseInstComment tries to parse a gowt instantiation directive.
//
// Returns (spec, true, nil) on a match, (_, false, nil) if not a gowt comment,
// or (_, _, err) if the comment looks like a gowt directive but is malformed.
func parseInstComment(text string) (InstantiationSpec, bool, error) {
	s := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "//"))
	if !strings.HasPrefix(s, "gowt:") {
		return InstantiationSpec{}, false, nil
	}
	s = strings.TrimSpace(s[len("gowt:"):])
	if !strings.HasPrefix(s, "type ") {
		return InstantiationSpec{}, false, nil
	}
	s = strings.TrimSpace(s[len("type "):])

	p := &directiveParser{s: s}
	spec, err := p.parse()
	if err != nil {
		return InstantiationSpec{}, false, err
	}
	return spec, true, nil
}

// ---------------------------------------------------------------------------
// directiveParser — cursor-based parser for gowt directives
// ---------------------------------------------------------------------------
//
// Grammar (after "gowt: type" prefix has been stripped):
//
//   directive   := NAME "wrapping" tmpl_ref "[" type_args "]" [using] [renaming]
//   tmpl_ref    := [qualified_ref "."] NAME
//   type_args   := base_arg ("," extra_arg)*
//   base_arg    := ["*"] qualified_ref "." NAME ["via" ctor_ref]
//   ctor_ref    := qualified_ref "." NAME | NAME
//   extra_arg   := ["*"] qualified_ref "." NAME
//   using       := "using" using_entry ("," using_entry)*
//   using_entry := NAME "=" pkg_ref | pkg_ref
//   renaming    := "renaming" rename ("," rename)*
//   rename      := NAME "=" NAME
//   pkg_ref     := NAME ("/" NAME)*  |  NAME ("." NAME)*
//
// ---------------------------------------------------------------------------

type directiveParser struct {
	s        string // remaining input (consumed left-to-right)
	typeName string // set early, used in error messages
}

func (p *directiveParser) errorf(format string, args ...any) error {
	if p.typeName != "" {
		return fmt.Errorf("type %s: "+format, append([]any{p.typeName}, args...)...)
	}
	return fmt.Errorf(format, args...)
}

// trim strips leading whitespace.
func (p *directiveParser) trim() { p.s = strings.TrimSpace(p.s) }

// done returns true when all input has been consumed.
func (p *directiveParser) done() bool { p.trim(); return p.s == "" }

// tryKeyword consumes keyword followed by whitespace.  Returns false
// (without advancing) when the keyword is not present.
func (p *directiveParser) tryKeyword(kw string) bool {
	p.trim()
	if !strings.HasPrefix(p.s, kw) {
		return false
	}
	after := p.s[len(kw):]
	if after != "" && after[0] != ' ' && after[0] != '\t' {
		return false
	}
	p.s = strings.TrimSpace(after)
	return true
}

// expectKeyword consumes keyword or returns an error.
func (p *directiveParser) expectKeyword(kw string) error {
	if !p.tryKeyword(kw) {
		return p.errorf("expected %q, got %q", kw, p.preview())
	}
	return nil
}

// word returns the next whitespace/delimiter-free token.
// delims are additional stop characters beyond whitespace.
func (p *directiveParser) word(delims string) (string, error) {
	p.trim()
	stop := " \t" + delims
	i := strings.IndexAny(p.s, stop)
	if i == 0 || p.s == "" {
		return "", p.errorf("expected identifier, got %q", p.preview())
	}
	var w string
	if i < 0 {
		w, p.s = p.s, ""
	} else {
		w, p.s = p.s[:i], p.s[i:]
	}
	return w, nil
}

// bracketed consumes "[" content "]" and returns the content.
func (p *directiveParser) bracketed() (string, error) {
	p.trim()
	if p.s == "" || p.s[0] != '[' {
		return "", p.errorf("expected '[', got %q", p.preview())
	}
	p.s = p.s[1:]
	close := strings.LastIndex(p.s, "]")
	if close < 0 {
		return "", p.errorf("missing ']'")
	}
	content := strings.TrimSpace(p.s[:close])
	p.s = strings.TrimSpace(p.s[close+1:])
	return content, nil
}

// until returns everything up to (but not including) the first occurrence
// of any character in delims, and advances past it.  If none found,
// returns the rest and empties the input.
func (p *directiveParser) until(delims string) string {
	p.trim()
	i := strings.IndexAny(p.s, delims)
	if i < 0 {
		r := p.s
		p.s = ""
		return strings.TrimSpace(r)
	}
	r := strings.TrimSpace(p.s[:i])
	p.s = p.s[i:]
	return r
}

// preview returns a short excerpt of the remaining input for error messages.
func (p *directiveParser) preview() string {
	p.trim()
	if len(p.s) <= 30 {
		return p.s
	}
	return p.s[:30] + "..."
}

// ---------------------------------------------------------------------------
// grammar productions
// ---------------------------------------------------------------------------

// parse is the top-level entry point.
// Input: NAME "wrapping" ... (the "gowt: type " prefix is already stripped).
func (p *directiveParser) parse() (InstantiationSpec, error) {
	typeName, err := p.word("[")
	if err != nil {
		return InstantiationSpec{}, err
	}
	p.typeName = typeName

	if err := p.expectKeyword("wrapping"); err != nil {
		return InstantiationSpec{}, err
	}

	tmplPkg, tmplName, err := p.parseTemplateRef()
	if err != nil {
		return InstantiationSpec{}, err
	}

	typeArgStr, err := p.bracketed()
	if err != nil {
		return InstantiationSpec{}, err
	}

	basePtr, basePkg, baseType, ctorPkg, ctorFunc, extraArgs, err := p.parseTypeArgs(typeArgStr)
	if err != nil {
		return InstantiationSpec{}, err
	}

	usingPkgs, err := p.parseUsing()
	if err != nil {
		return InstantiationSpec{}, err
	}

	renames, err := p.parseRenaming()
	if err != nil {
		return InstantiationSpec{}, err
	}

	return InstantiationSpec{
		TypeName:      typeName,
		TmplPkg:       tmplPkg,
		TmplName:      tmplName,
		BasePtr:       basePtr,
		BasePkgAlias:  basePkg,
		BaseTypeName:  baseType,
		CtorPkgAlias:  ctorPkg,
		CtorFuncName:  ctorFunc,
		ExtraTypeArgs: extraArgs,
		UsingPkgs:     usingPkgs,
		Renames:       renames,
	}, nil
}

// parseTemplateRef parses [pkg.]Template up to the opening '['.
func (p *directiveParser) parseTemplateRef() (pkg, name string, err error) {
	ref := p.until("[")
	if ref == "" {
		return "", "", p.errorf("missing template reference")
	}
	if dot := strings.LastIndex(ref, "."); dot >= 0 {
		return ref[:dot], ref[dot+1:], nil
	}
	return "", ref, nil
}

// parseTypeArgs parses the comma-separated content inside [...].
//
//	base_arg := ["*"] Pkg.Type ["via" [Pkg.]Func]
//	extra_arg := ["*"] Pkg.Type
func (p *directiveParser) parseTypeArgs(s string) (
	basePtr bool, basePkg, baseType, ctorPkg, ctorFunc string,
	extra []ExtraTypeArg, err error,
) {
	parts := strings.Split(s, ",")

	// First element: base type + optional via.
	basePtr, basePkg, baseType, ctorPkg, ctorFunc, err = p.parseBaseArg(strings.TrimSpace(parts[0]))
	if err != nil {
		return
	}

	// Remaining elements: extra type arguments.
	for _, raw := range parts[1:] {
		arg, e := p.parseExtraArg(strings.TrimSpace(raw))
		if e != nil {
			err = e
			return
		}
		extra = append(extra, arg)
	}
	return
}

// parseBaseArg parses "[*]Pkg.Type [via [Pkg.]Func]".
func (p *directiveParser) parseBaseArg(s string) (ptr bool, basePkg, baseType, ctorPkg, ctorFunc string, err error) {
	var ctorRef string
	if parts := strings.SplitN(s, " via ", 2); len(parts) == 2 {
		s = strings.TrimSpace(parts[0])
		ctorRef = strings.TrimSpace(parts[1])
	}

	ptr, basePkg, baseType, err = parseTypeRef(s)
	if err != nil {
		return false, "", "", "", "", p.errorf("base type: %w", err)
	}

	ctorPkg, ctorFunc, err = p.resolveCtor(ctorRef, basePkg)
	return
}

// resolveCtor resolves the constructor reference.
// Empty ctorRef → basePkg.New.  Unqualified name → basePkg.<name>.
func (p *directiveParser) resolveCtor(ctorRef, basePkg string) (string, string, error) {
	if ctorRef == "" {
		return basePkg, "New", nil
	}
	if strings.Contains(ctorRef, ".") {
		pkg, name, err := splitQualified(ctorRef)
		if err != nil {
			return "", "", p.errorf("ctor: %w", err)
		}
		return pkg, name, nil
	}
	return basePkg, ctorRef, nil
}

// parseExtraArg parses "[*]Pkg.Type".
func (p *directiveParser) parseExtraArg(s string) (ExtraTypeArg, error) {
	if s == "" {
		return ExtraTypeArg{}, p.errorf("empty extra type argument")
	}
	ptr, pkg, name, err := parseTypeRef(s)
	if err != nil {
		return ExtraTypeArg{}, p.errorf("extra type arg: %w", err)
	}
	return ExtraTypeArg{Ptr: ptr, PkgAlias: pkg, TypeName: name}, nil
}

// parseUsing optionally consumes "using entry, entry, ...".
func (p *directiveParser) parseUsing() (map[string]string, error) {
	if !p.tryKeyword("using") {
		return nil, nil
	}

	// Collect everything until "renaming" or end-of-input.
	// Entries are comma-separated and do not contain spaces (import paths).
	var raw string
	if ri := strings.Index(p.s, " renaming"); ri >= 0 {
		raw = strings.TrimSpace(p.s[:ri])
		p.s = strings.TrimSpace(p.s[ri+1:])
	} else {
		raw = strings.TrimSpace(p.s)
		p.s = ""
	}

	pkgs := make(map[string]string)
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if eq := strings.Index(entry, "="); eq >= 0 {
			name := strings.TrimSpace(entry[:eq])
			pkg := strings.TrimSpace(entry[eq+1:])
			if name == "" || pkg == "" {
				return nil, p.errorf("using: name and package must not be empty in %q", entry)
			}
			pkgs[name] = pkg
		} else {
			if _, dup := pkgs[""]; dup {
				return nil, p.errorf("using: only one default package (without name=) is allowed")
			}
			pkgs[""] = entry
		}
	}
	return pkgs, nil
}

// parseRenaming optionally consumes "renaming Old=New, ...".
func (p *directiveParser) parseRenaming() (map[string]string, error) {
	if !p.tryKeyword("renaming") {
		return nil, nil
	}

	renames := make(map[string]string)
	for _, pair := range strings.Split(p.s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		eq := strings.Index(pair, "=")
		if eq < 0 {
			return nil, p.errorf("renaming %q: expected OldName=NewName", pair)
		}
		old := strings.TrimSpace(pair[:eq])
		newName := strings.TrimSpace(pair[eq+1:])
		if old == "" || newName == "" {
			return nil, p.errorf("renaming %q: names must not be empty", pair)
		}
		renames[old] = newName
	}
	p.s = ""
	return renames, nil
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// parseTypeRef parses "[*]Pkg.Type" into (pointer, package, name).
func parseTypeRef(s string) (ptr bool, pkg, name string, err error) {
	if strings.HasPrefix(s, "*") {
		ptr = true
		s = s[1:]
	}
	pkg, name, err = splitQualified(s)
	return
}

// splitQualified splits "pkg.Name" at the last dot.
func splitQualified(s string) (pkg, name string, err error) {
	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return "", "", fmt.Errorf("%q must be a qualified name (pkg.Name)", s)
	}
	return s[:dot], s[dot+1:], nil
}
