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

	// Resolve optional default "using" package (PACKAGE. placeholder).
	var usingAlias, usingImportPath string
	if defaultPkg, ok := inst.UsingPkgs[""]; ok {
		usingImportPath, usingAlias, err = resolveImportRef(info, defaultPkg)
		if err != nil {
			return nil, fmt.Errorf("resolving using package %q: %w", defaultPkg, err)
		}
	}

	// Validate extra type arg count against template type params.
	if len(inst.ExtraTypeArgs) != len(decl.TypeParams)-1 {
		return nil, fmt.Errorf("template %s has %d type parameters, but directive provides %d type arguments",
			inst.TmplName, len(decl.TypeParams), 1+len(inst.ExtraTypeArgs))
	}

	// Resolve extra type arguments and build ExtraSubst list.
	var extraSubsts []ExtraSubst
	var extraImportPaths []string
	for i, eta := range inst.ExtraTypeArgs {
		importPath, alias, err := resolveImportRef(info, eta.PkgAlias)
		if err != nil {
			return nil, fmt.Errorf("resolving extra type arg #%d package %q: %w", i+2, eta.PkgAlias, err)
		}
		if importPath != "" {
			extraImportPaths = append(extraImportPaths, importPath)
		}
		extraSubsts = append(extraSubsts, ExtraSubst{
			TypeParam: decl.TypeParams[i+1].Name,
			Pkg:       alias,
			TypeName:  eta.TypeName,
			Ptr:       eta.Ptr,
		})
	}

	// Resolve gowt.* placeholder imports via named "using" entries.
	for _, ph := range gf.PlaceholderImports {
		realPkg, ok := inst.UsingPkgs[ph.Name]
		if !ok {
			return nil, fmt.Errorf("template uses placeholder import %q but no using mapping for %q was provided in the directive",
				"gowt."+ph.Name, ph.Name)
		}
		phImportPath, phAlias, err := resolveImportRef(info, realPkg)
		if err != nil {
			return nil, fmt.Errorf("resolving using package %q for placeholder %q: %w", realPkg, ph.Name, err)
		}
		// If the resolved alias differs from the template alias, we need
		// an explicit import alias so that the template code's qualifiers
		// (which use ph.Alias) continue to work.
		imp := ImportSpec{Path: phImportPath}
		if phAlias != ph.Alias {
			imp.Alias = ph.Alias
		}
		gf.Imports = append(gf.Imports, imp)
	}

	// Build the import set for the generated file.
	importMap := make(map[string]ImportSpec)
	for _, imp := range gf.Imports {
		importMap[imp.Path] = imp
	}
	addImport := func(p string) {
		if p != "" {
			if _, exists := importMap[p]; !exists {
				importMap[p] = ImportSpec{Path: p}
			}
		}
	}
	addImport(baseImportPath)
	addImport(ctorImportPath)
	addImport(usingImportPath)
	for _, p := range extraImportPaths {
		addImport(p)
	}
	var imports []ImportSpec
	for _, imp := range importMap {
		imports = append(imports, imp)
	}

	// Apply AST-based substitutions to the template declarations.
	ctor := ctorLocalAlias + "." + inst.CtorFuncName
	substituted, err := substituteDecls(gf, SubstParams{
		TmplName:    decl.Name,
		TypeParam:   decl.TypeParams[0].Name,
		TargetName:  inst.TypeName,
		BasePkg:     basePkgAlias,
		BaseType:    inst.BaseTypeName,
		BasePtr:     inst.BasePtr,
		Ctor:        ctor,
		ExtraParams: extraSubsts,
		Renames:     inst.Renames,
		UsingAlias:  usingAlias,
	})
	if err != nil {
		return nil, fmt.Errorf("substitution: %w", err)
	}

	// Build compile-time constraint assertion when at least one type
	// parameter has a non-trivial constraint.
	var constraintAssertion string
	hasNonTrivial := false
	for _, tp := range decl.TypeParams {
		if tp.Constraint != "" && tp.Constraint != "any" {
			hasNonTrivial = true
			break
		}
	}
	if hasNonTrivial {
		var checkParams, checkArgs []string
		for i, tp := range decl.TypeParams {
			checkParams = append(checkParams, fmt.Sprintf("T%d %s", i, tp.Constraint))
			if i == 0 {
				expr := basePkgAlias + "." + inst.BaseTypeName
				if inst.BasePtr {
					expr = "*" + expr
				}
				checkArgs = append(checkArgs, expr)
			} else {
				es := extraSubsts[i-1]
				expr := es.Pkg + "." + es.TypeName
				if es.Ptr {
					expr = "*" + expr
				}
				checkArgs = append(checkArgs, expr)
			}
		}
		constraintAssertion = fmt.Sprintf(
			"type _gowtCheck%s[%s] struct{}\nvar _ _gowtCheck%s[%s]",
			inst.TypeName, strings.Join(checkParams, ", "),
			inst.TypeName, strings.Join(checkArgs, ", "),
		)
	}

	ctx := genContext{
		Package:             info.Package,
		Imports:             imports,
		SourceFile:          filepath.Base(tmplPath),
		ConstraintAssertion: constraintAssertion,
		Decls:               substituted,
	}

	out, err := generateSource(ctx)
	if err != nil {
		return nil, err
	}

	outPath := filepath.Join(info.Dir, strings.ToLower(inst.TypeName)+"_gowtgen.go")
	return &GenerationResult{Path: outPath, Content: out}, nil
}

