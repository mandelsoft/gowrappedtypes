package gowt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mandelsoft/vfs/pkg/vfs"
)

// GenerationResult holds the output of one instantiation directive.
type GenerationResult struct {
	Path    string // intended output file path
	Content []byte // gofmt-formatted Go source
}

// GenerateGoFile scans path for instantiation directives using fs and returns
// the generated content for each one.  It does not write anything to disk.
func GenerateGoFile(fs vfs.FileSystem, path string) ([]GenerationResult, error) {
	info, err := scanGoFile(fs, path)
	if err != nil {
		return nil, err
	}
	var results []GenerationResult
	for _, inst := range info.Instantiations {
		r, err := generateInstantiation(info, inst)
		if err != nil {
			return nil, fmt.Errorf("type %s: %w", inst.TypeName, err)
		}
		results = append(results, *r)
	}
	return results, nil
}

// ProcessGoFile generates all instantiations for path and writes each result
// to its output file via fs.
func ProcessGoFile(fs vfs.FileSystem, path string) error {
	results, err := GenerateGoFile(fs, path)
	if err != nil {
		return err
	}
	for _, r := range results {
		if err := vfs.WriteFile(fs, r.Path, r.Content, 0644); err != nil {
			return fmt.Errorf("write %s: %w", r.Path, err)
		}
		fmt.Printf("gowt: wrote %s\n", r.Path)
	}
	return nil
}

// generateInstantiation runs the full substitution pipeline for one
// instantiation directive and returns the intended output path together with
// the gofmt-formatted source.  It does not write anything to disk.
func generateInstantiation(info *GoFileInfo, inst InstantiationSpec) (*GenerationResult, error) {
	// Locate the _gowt.go template.
	tmplPath, gf, decl, err := findTemplate(info.FS, info.Dir, inst)
	if err != nil {
		return nil, err
	}

	// Resolve base type: import path + local alias.
	baseImportPath, basePkgAlias, err := resolveImportRef(info, inst.BasePkgAlias)
	if err != nil {
		return nil, fmt.Errorf("resolving base package %q: %w", inst.BasePkgAlias, err)
	}

	// Resolve ctor package: import path + local alias.
	ctorImportPath, ctorLocalAlias, err := resolveImportRef(info, inst.CtorPkgAlias)
	if err != nil {
		return nil, fmt.Errorf("resolving ctor package %q: %w", inst.CtorPkgAlias, err)
	}

	// Resolve template package for cross-package instantiation.
	// The template package import is not added to the generated file —
	// it is only needed at generation time to locate the _gowt.go file.
	if inst.TmplPkg != "" {
		// resolveImportRef validates the package reference; we don't need
		// the import path or alias in the generated output.
		if _, _, e := resolveImportRef(info, inst.TmplPkg); e != nil {
			// Fall through: findTemplate will resolve via go list.
		}
	}

	// Resolve optional "using" package.
	var usingAlias, usingImportPath string
	if inst.UsingPkg != "" {
		usingImportPath, usingAlias, err = resolveImportRef(info, inst.UsingPkg)
		if err != nil {
			return nil, fmt.Errorf("resolving using package %q: %w", inst.UsingPkg, err)
		}
	}

	// Build the import set for the generated file.
	importMap := make(map[string]ImportSpec)
	for _, imp := range gf.Imports {
		importMap[imp.Path] = imp
	}
	for _, p := range []string{baseImportPath, ctorImportPath, usingImportPath} {
		if p != "" {
			if _, exists := importMap[p]; !exists {
				importMap[p] = ImportSpec{Path: p}
			}
		}
	}
	var imports []ImportSpec
	for _, imp := range importMap {
		imports = append(imports, imp)
	}

	// Apply AST-based substitutions to the template declarations.
	ctor := ctorLocalAlias + "." + inst.CtorFuncName
	substituted, err := substituteDecls(gf, SubstParams{
		TmplName:   decl.Name,
		TypeParam:  decl.TypeParam,
		TargetName: inst.TypeName,
		BasePkg:    basePkgAlias,
		BaseType:   inst.BaseTypeName,
		BasePtr:    inst.BasePtr,
		Ctor:       ctor,
		Renames:    inst.Renames,
		UsingAlias: usingAlias,
	})
	if err != nil {
		return nil, fmt.Errorf("substitution: %w", err)
	}

	ctx := genContext{
		Package:    info.Package,
		Imports:    imports,
		SourceFile: filepath.Base(tmplPath),
		Decls:      substituted,
	}

	out, err := generateSource(ctx)
	if err != nil {
		return nil, err
	}

	outPath := filepath.Join(info.Dir, strings.ToLower(inst.TypeName)+"_gowtgen.go")
	return &GenerationResult{Path: outPath, Content: out}, nil
}
