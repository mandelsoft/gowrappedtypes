// gowt generates concrete Go types from parameterized _gowt.go template files.
//
// # Template files
//
// A _gowt.go file is written in regular Go generics syntax and is parseable by
// go/parser.  It carries a "//go:build ignore" constraint so the Go toolchain
// excludes it from normal builds but still includes it in module downloads.
// It defines a struct type with a single type parameter that represents the
// embedded base type, and optionally methods on that type.
//
// Three special identifiers are recognised anywhere in a template:
//
//   - NEW is replaced by the constructor named in the "via" clause of the
//     instantiation directive.
//   - The template type name itself (and any camelCase-prefixed form of it)
//     is replaced by the generated type name; see "Name conventions" below.
//   - PACKAGE. (always written with a trailing dot) is replaced by the alias
//     of the package named in the "using" clause, or removed entirely when no
//     "using" clause is present; see "Substitution pipeline" step 7.
//
// Example template (environment_gowt.go):
//
//	package mypkg
//
//	import "github.com/example/epi"
//
//	type Environment[Base epi.Environment] struct {
//	    Base
//	    ExtraGroup
//	}
//
//	func New[Base epi.Environment](opts ...epi.Option) (*Environment[Base], error) {
//	    e := epi.NewEnvState(opts...)
//	    base, err := NEW(e)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return &Environment[Base]{Base: base, ExtraGroup: *NewGroup(e)}, nil
//	}
//
// # Instantiation directives
//
// Instantiation is triggered by a specially formatted comment in any .go file
// in the target package:
//
//	// gowt: type GeneratedName wrapping [pkg.]TemplateName[BasePkg.BaseType via CtorPkg.CtorFunc]
//
// The base type may be written as a pointer type:
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment via common.New]
//
// An optional "using" clause names a package whose import is added to the
// generated file and whose alias replaces the PACKAGE. placeholder in the
// template (see "Substitution pipeline").  When "using" is omitted, PACKAGE.
// is removed, leaving an unqualified identifier suitable for the local package:
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] using mypkg
//
// An optional "renaming" clause maps template function names to new names in
// the generated output:
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] renaming New=NewMyEnv
//
// Multiple renames are comma-separated.  Both clauses may be combined, with
// "using" preceding "renaming":
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] using mypkg renaming New=NewMyEnv,Wrap=BuildMyEnv
//
// The template may live in a different package; use its import path or a local
// alias as the package qualifier:
//
//	// gowt: type MyEnv wrapping mypkg.Environment[*common.Environment via common.New]
//
// # Substitution pipeline
//
// For each declaration in the template the following substitutions are applied
// in order:
//
//  0. Explicit renames (from the "renaming" clause): every word-boundary
//     occurrence of OldName is replaced by NewName, covering both the
//     declaration and any call sites within the template body.
//  1. [prefix]TemplateName[...] → [prefix]GeneratedName  (type expressions
//     with type-argument lists; the optional word prefix is preserved so that
//     e.g. "NewEnvironment[...]" becomes "NewMyEnv" — see "Name conventions").
//  2. func F[TypeParam ...] → func F  (removes type parameters from function
//     signatures).
//  3. TypeParam: → BaseType:  (struct-literal field keys; must precede step 4).
//  4. TypeParam → [*]BasePkg.BaseType  (all remaining occurrences in type
//     expressions; the "*" prefix is added when the instantiation uses a
//     pointer base type).
//  5. Bare TemplateName → GeneratedName  (references without type-argument
//     lists, with the same camelCase-prefix preservation as step 1).
//  6. NEW → CtorPkg.CtorFunc.
//  7. PACKAGE. → usingAlias. (named package) or "" (local package — no qualifier needed in Go).
//
// # Name conventions
//
// When multiple instantiations of the same template coexist in one package,
// every instantiation would produce a function named "New", causing a compile
// error.  To avoid this, a template author may suffix function names with the
// template type name:
//
//	func NewEnvironment[Base epi.Environment](...) (*Environment[Base], error) { ... }
//
// The substitution pipeline treats any word immediately preceding the template
// type name as a preserved prefix, so "NewEnvironment" becomes "NewMyEnv" when
// the generated type is "MyEnv".  This is optional: the plain "func New" style
// continues to work for single-instantiation cases.
//
// Alternatively, the "renaming" clause can rename functions to arbitrary names
// regardless of the template type name.
//
// # Output
//
// The generated file is written to <lowercase(GeneratedName)>_gowtgen.go in the
// same directory as the .go file containing the directive.
//
// # Usage
//
//	//go:generate go run github.com/mandelsoft/gowrappedtypes
package main

import (
	"fmt"
	"os"

	"github.com/mandelsoft/vfs/pkg/osfs"

	"github.com/mandelsoft/gowrappedtypes/gowt"
)

func main() {
	files := os.Args[1:]
	if len(files) == 0 {
		if gf := os.Getenv("GOFILE"); gf != "" {
			files = []string{gf}
		} else {
			fmt.Fprintln(os.Stderr, "gowt: GOFILE not set; pass the target .go file as an argument")
			os.Exit(1)
		}
	}

	var failed bool
	for _, f := range files {
		if err := gowt.ProcessGoFile(osfs.OsFs, f); err != nil {
			fmt.Fprintf(os.Stderr, "gowt: %s: %v\n", f, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
