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
	UsingPkgs     map[string]string  // using clause mappings: "" → PACKAGE. default, "name" → gowt.<name>
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

// parseInstComment tries to parse:
//
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType]
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType via CtorFunc]
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType via CtorPkg.CtorFunc]
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType via CtorPkg.CtorFunc] using <pkg>
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType via CtorPkg.CtorFunc] using <pkg> renaming OldName=NewName,...
//	// gowt: type X wrapping [pkg.]Template[BasePkg.BaseType via CtorPkg.CtorFunc] renaming OldName=NewName,...
//
// When "via" is omitted, the constructor defaults to BasePkg.New.
// When "via" names an unqualified function (no dot), the base package is used.
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

	// TypeName
	typeName, s, ok := cutWord(s)
	if !ok {
		return InstantiationSpec{}, false, nil
	}

	if !strings.HasPrefix(s, "wrapping ") {
		return InstantiationSpec{}, false, nil
	}
	s = strings.TrimSpace(s[len("wrapping "):])

	// Split template reference from type argument at the first '['.
	bOpen := strings.Index(s, "[")
	if bOpen < 0 {
		return InstantiationSpec{}, false, fmt.Errorf("type %s wrapping: missing '[' in instantiation", typeName)
	}
	tmplRef := strings.TrimSpace(s[:bOpen])
	s = s[bOpen+1:]

	bClose := strings.LastIndex(s, "]")
	if bClose < 0 {
		return InstantiationSpec{}, false, fmt.Errorf("type %s wrapping: missing ']'", typeName)
	}
	typeArg := strings.TrimSpace(s[:bClose])
	tail := strings.TrimSpace(s[bClose+1:])

	// Parse optional "using" clause before "renaming".
	// Entries are comma-separated: bare pkg → PACKAGE. default, name=pkg → gowt.<name>.
	var usingPkgs map[string]string
	if strings.HasPrefix(tail, "using ") {
		tail = strings.TrimSpace(tail[len("using "):])
		// Separate using entries from optional renaming clause.
		var usingRaw string
		if ri := strings.Index(tail, " renaming"); ri >= 0 {
			usingRaw = strings.TrimSpace(tail[:ri])
			tail = strings.TrimSpace(tail[ri+1:])
		} else {
			usingRaw = strings.TrimSpace(tail)
			tail = ""
		}
		usingPkgs = make(map[string]string)
		for _, entry := range strings.Split(usingRaw, ",") {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			if eq := strings.Index(entry, "="); eq >= 0 {
				name := strings.TrimSpace(entry[:eq])
				pkg := strings.TrimSpace(entry[eq+1:])
				if name == "" || pkg == "" {
					return InstantiationSpec{}, false, fmt.Errorf("type %s: using: name and package must not be empty in %q", typeName, entry)
				}
				usingPkgs[name] = pkg
			} else {
				if _, exists := usingPkgs[""]; exists {
					return InstantiationSpec{}, false, fmt.Errorf("type %s: using: only one default package (without name=) is allowed", typeName)
				}
				usingPkgs[""] = entry
			}
		}
	}

	// Parse optional "renaming OldName=NewName,..." after the closing ']'.
	var renames map[string]string
	if strings.HasPrefix(tail, "renaming ") {
		tail = strings.TrimSpace(tail[len("renaming "):])
		renames = make(map[string]string)
		for _, pair := range strings.Split(tail, ",") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			eq := strings.Index(pair, "=")
			if eq < 0 {
				return InstantiationSpec{}, false, fmt.Errorf("type %s: renaming %q: expected OldName=NewName", typeName, pair)
			}
			old := strings.TrimSpace(pair[:eq])
			newName := strings.TrimSpace(pair[eq+1:])
			if old == "" || newName == "" {
				return InstantiationSpec{}, false, fmt.Errorf("type %s: renaming %q: names must not be empty", typeName, pair)
			}
			renames[old] = newName
		}
	}

	// Parse tmplRef → TmplPkg (optional) + TmplName.
	var tmplPkg, tmplName string
	if dot := strings.LastIndex(tmplRef, "."); dot >= 0 {
		tmplPkg = tmplRef[:dot]
		tmplName = tmplRef[dot+1:]
	} else {
		tmplName = tmplRef
	}

	// Split type arguments on commas.  The first element is the base type
	// with an optional "via" clause; subsequent elements are extra type args.
	argParts := strings.Split(typeArg, ",")
	firstArg := strings.TrimSpace(argParts[0])

	// Parse firstArg: "BasePkg.BaseType [via [CtorPkg.]CtorFunc]"
	var baseRef, ctorRef string
	viaParts := strings.SplitN(firstArg, " via ", 2)
	baseRef = strings.TrimSpace(viaParts[0])
	if len(viaParts) == 2 {
		ctorRef = strings.TrimSpace(viaParts[1])
	}

	basePtr := strings.HasPrefix(baseRef, "*")
	if basePtr {
		baseRef = baseRef[1:]
	}
	basePkg, baseType, err := splitDot(baseRef)
	if err != nil {
		return InstantiationSpec{}, false, fmt.Errorf("type %s: base type: %w", typeName, err)
	}

	// Resolve constructor: default to basePkg.New when via is omitted;
	// use basePkg when via names an unqualified function.
	var ctorPkg, ctorFunc string
	if ctorRef == "" {
		ctorPkg = basePkg
		ctorFunc = "New"
	} else if strings.Contains(ctorRef, ".") {
		ctorPkg, ctorFunc, err = splitDot(ctorRef)
		if err != nil {
			return InstantiationSpec{}, false, fmt.Errorf("type %s: ctor: %w", typeName, err)
		}
	} else {
		ctorPkg = basePkg
		ctorFunc = ctorRef
	}

	// Parse extra type arguments (everything after the first comma).
	var extraTypeArgs []ExtraTypeArg
	for _, raw := range argParts[1:] {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return InstantiationSpec{}, false, fmt.Errorf("type %s: empty extra type argument", typeName)
		}
		ptr := strings.HasPrefix(raw, "*")
		if ptr {
			raw = raw[1:]
		}
		pkg, name, err := splitDot(raw)
		if err != nil {
			return InstantiationSpec{}, false, fmt.Errorf("type %s: extra type arg: %w", typeName, err)
		}
		extraTypeArgs = append(extraTypeArgs, ExtraTypeArg{Ptr: ptr, PkgAlias: pkg, TypeName: name})
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
		ExtraTypeArgs: extraTypeArgs,
		UsingPkgs:     usingPkgs,
		Renames:       renames,
	}, true, nil
}

// cutWord returns the first whitespace-delimited token and the remainder.
func cutWord(s string) (word, rest string, ok bool) {
	s = strings.TrimSpace(s)
	i := strings.IndexAny(s, " \t\n\r")
	if i < 0 {
		if s != "" {
			return s, "", true
		}
		return "", "", false
	}
	return s[:i], strings.TrimSpace(s[i:]), true
}

// splitDot splits "pkg.Name" into ("pkg", "Name"). Returns an error if there is no dot.
func splitDot(s string) (pkg, name string, err error) {
	dot := strings.LastIndex(s, ".")
	if dot < 0 {
		return "", "", fmt.Errorf("%q must be a qualified name (pkg.Name)", s)
	}
	return s[:dot], s[dot+1:], nil
}
