package gowt

import (
	"fmt"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mandelsoft/vfs/pkg/vfs"
)

// findTemplate locates the _gowt.go file that contains a type named inst.TmplName,
// parses it, and returns the path, the file, and the matching declaration.
//
// Template files use a _gowt.go suffix and carry a "//go:build ignore" constraint
// so that the Go toolchain excludes them from normal builds but still includes them
// in module downloads.
//
// When inst.TmplPkg is empty the template is looked up in gofileDir via fs.
// Otherwise inst.TmplPkg is resolved to a directory via go list (OS-level),
// and that directory is read from the OS filesystem.
func findTemplate(fs vfs.FileSystem, gofileDir string, inst InstantiationSpec) (string, *GowtFile, *GowtDecl, error) {
	var dir string
	var dirFS vfs.FileSystem
	if inst.TmplPkg == "" {
		dir = gofileDir
		dirFS = fs
	} else {
		var err error
		dir, err = packageDir(inst.TmplPkg)
		if err != nil {
			return "", nil, nil, fmt.Errorf("resolving package %q: %w", inst.TmplPkg, err)
		}
		dirFS = fs // cross-package: still use same fs (typically osfs in production)
	}

	entries, err := vfs.ReadDir(dirFS, dir)
	if err != nil {
		return "", nil, nil, fmt.Errorf("reading %s: %w", dir, err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), "_gowt.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		raw, err := vfs.ReadFile(dirFS, path)
		if err != nil {
			continue
		}
		gf, err := parseGowtFile(path, string(raw))
		if err != nil {
			continue
		}
		for i := range gf.Decls {
			if gf.Decls[i].Name == inst.TmplName {
				return path, gf, &gf.Decls[i], nil
			}
		}
	}
	return "", nil, nil, fmt.Errorf("template type %q not found in any _gowt.go file under %s",
		inst.TmplName, dir)
}

// packageDir returns the filesystem directory for the given Go import path.
func packageDir(importPath string) (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", importPath).Output()
	if err != nil {
		return "", fmt.Errorf("go list %s: %w", importPath, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// resolveImportRef resolves a package reference to (importPath, localAlias).
//
//   - Full import path (contains "/"): used directly; localAlias = last path element.
//   - Short alias: looked up in info.Imports, then in all .go files in the package dir
//     via info.FS.
func resolveImportRef(info *GoFileInfo, ref string) (importPath, localAlias string, err error) {
	if strings.Contains(ref, "/") {
		parts := strings.Split(ref, "/")
		return ref, parts[len(parts)-1], nil
	}
	if path, ok := info.Imports[ref]; ok {
		return path, ref, nil
	}
	// Fall back: scan all .go files in the same directory via info.FS.
	entries, _ := vfs.ReadDir(info.FS, info.Dir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		src, ferr := vfs.ReadFile(info.FS, filepath.Join(info.Dir, e.Name()))
		if ferr != nil {
			continue
		}
		fset := token.NewFileSet()
		f, ferr := parser.ParseFile(fset, filepath.Join(info.Dir, e.Name()), src, 0)
		if ferr != nil {
			continue
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
			if alias == ref {
				return ipath, ref, nil
			}
		}
	}
	return "", "", fmt.Errorf("import alias %q not found in package %s; use a full import path in the wrapping comment",
		ref, info.Dir)
}
