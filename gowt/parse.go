package gowt

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// ImportSpec represents one import declaration.
type ImportSpec struct {
	Alias string // explicit alias; empty = use last path element
	Path  string // import path without quotes
}

// TypeParamDecl describes one type parameter of a template type.
type TypeParamDecl struct {
	Name       string // type parameter name (e.g. "Base")
	Constraint string // constraint expression text (e.g. "epi.Environment")
}

// GowtDecl describes one parameterized type template found in a _gowt.go file.
type GowtDecl struct {
	Name       string          // template type name (e.g. "Environment")
	TypeParams []TypeParamDecl // index 0 = embedded base type param; rest = extra
}

// PlaceholderImport describes a gowt.* import in a template file.
// These are not real Go imports but placeholders resolved via the "using" clause.
type PlaceholderImport struct {
	Name  string // name from gowt.<name> (e.g. "registry")
	Alias string // qualifier used in template code (= Name when no explicit alias)
}

// GowtFile is the result of parsing a _gowt.go template file.
type GowtFile struct {
	Filename            string
	Package             string
	Imports             []ImportSpec        // real imports (excludes gowt.* placeholders)
	PlaceholderImports  []PlaceholderImport // gowt.* placeholder imports
	Decls               []GowtDecl          // one entry per parameterized type declaration
	File                *ast.File           // parsed AST; modified in place by substituteDecls
	Fset                *token.FileSet      // position information for File
}

// parseGowtFile parses a _gowt.go template file (written in Go generics syntax)
// and returns the structured representation together with the parsed AST.
func parseGowtFile(filename, src string) (*GowtFile, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(filename), err)
	}

	gf := &GowtFile{
		Filename: filename,
		Package:  f.Name.Name,
		File:     f,
		Fset:     fset,
	}

	// Collect imports, separating gowt.* placeholders from real imports.
	for _, imp := range f.Imports {
		ipath := strings.Trim(imp.Path.Value, `"`)
		var alias string
		if imp.Name != nil && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		}
		if strings.HasPrefix(ipath, "gowt.") {
			name := ipath[len("gowt."):]
			a := alias
			if a == "" {
				a = name
			}
			gf.PlaceholderImports = append(gf.PlaceholderImports, PlaceholderImport{Name: name, Alias: a})
		} else {
			gf.Imports = append(gf.Imports, ImportSpec{Alias: alias, Path: ipath})
		}
	}

	// Walk top-level declarations to collect parameterized type metadata.
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok || d.Tok != token.TYPE {
			continue
		}
		for _, spec := range d.Specs {
			ts := spec.(*ast.TypeSpec)
			if ts.TypeParams == nil || len(ts.TypeParams.List) == 0 {
				continue
			}
			gd, err := extractDeclMeta(fset, ts, src)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", filename, err)
			}
			gf.Decls = append(gf.Decls, *gd)
		}
	}

	return gf, nil
}

// extractDeclMeta extracts type parameter names and constraints from a generic TypeSpec.
func extractDeclMeta(fset *token.FileSet, ts *ast.TypeSpec, src string) (*GowtDecl, error) {
	if len(ts.TypeParams.List) == 0 {
		return nil, fmt.Errorf("type %s: expected at least one type parameter", ts.Name.Name)
	}

	tfile := fset.File(ts.Pos())
	srcBytes := []byte(src)

	var params []TypeParamDecl
	for _, tf := range ts.TypeParams.List {
		constraintText := strings.TrimSpace(
			string(srcBytes[tfile.Offset(tf.Type.Pos()):tfile.Offset(tf.Type.End())]),
		)
		for _, name := range tf.Names {
			params = append(params, TypeParamDecl{
				Name:       name.Name,
				Constraint: constraintText,
			})
		}
	}

	return &GowtDecl{
		Name:       ts.Name.Name,
		TypeParams: params,
	}, nil
}
