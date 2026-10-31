// gowt generates concrete Go types from parameterized _gowt.go template files.
//
// Go does not allow embedding a type parameter in a struct.  This means you
// cannot write a single generic decorator, wrapper, or layered type that embeds
// different base types; instead you must hand-write a separate struct (plus
// constructor, helpers, etc.) for every concrete base — even when the layout
// and body are structurally identical.  gowt solves this: you write one
// template in generic syntax and stamp out concrete, non-generic types with a
// one-line comment directive.
//
// # Template files
//
// A _gowt.go file is written in regular Go generics syntax and is parseable by
// go/parser.  It carries a "//go:build ignore" constraint so the compiler never
// sees the illegal type-parameter embedding, while go/parser (which does not
// type-check) can still parse it.  The .go extension ensures the file is
// included in module downloads, so cross-package templates work even from
// dependencies fetched via the module proxy.
//
// The template defines a struct type with a single type parameter that
// represents the embedded base type, together with any number of functions
// (generic or non-generic) that operate on that type.
//
// Three special identifiers are recognised anywhere in a template:
//
//   - NEW is replaced by the constructor named in the "via" clause of the
//     instantiation directive.
//   - *TemplateName* — any symbol containing the template type name gets that
//     portion replaced by the generated type name, preserving both prefix and
//     suffix; see "Name conventions" below.
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
//	// gowt: type GeneratedName wrapping [pkg.]TemplateName[BasePkg.BaseType]
//
// The "via" clause is optional; when omitted, NEW defaults to BasePkg.New.
// An unqualified function name uses the base type's package:
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment]
//	// gowt: type MyEnv wrapping Environment[*common.Environment via New]
//	// gowt: type MyEnv wrapping Environment[*common.Environment via common.New]
//
// The base type may be written as a pointer type:
//
//	// gowt: type MyEnv wrapping Environment[*common.Environment]
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
//  1. [prefix]*TemplateName*[suffix][...] → [prefix]*GeneratedName*[suffix]
//     (type expressions with type-argument lists; both prefix and suffix around
//     the template name are preserved — see "Name conventions").
//  2. func F[TypeParam ...] → func F  (removes type parameters from function
//     signatures).
//  3. TypeParam: → BaseType:  (struct-literal field keys; must precede step 4).
//  4. TypeParam → [*]BasePkg.BaseType  (all remaining occurrences in type
//     expressions; the "*" prefix is added when the instantiation uses a
//     pointer base type).
//  5. Bare *TemplateName* → *GeneratedName*  (references without type-argument
//     lists, with the same prefix/suffix preservation as step 1).
//  6. NEW → CtorPkg.CtorFunc.
//  7. PACKAGE. → usingAlias. (named package) or "" (local package — no qualifier needed in Go).
//
// # Name conventions
//
// Any symbol whose name contains the template type name gets that portion
// replaced by the generated type name, preserving both prefix and suffix.
// For example, with template type "Environment" and generated type "MyEnv":
//
//	NewEnvironment       → NewMyEnv         (prefix preserved)
//	EnvironmentConfig    → MyEnvConfig      (suffix preserved)
//	NewEnvironmentConfig → NewMyEnvConfig   (both preserved)
//
// This applies equally to generic and non-generic functions — for generic
// functions, the type parameter list is stripped first, then the name is
// substituted.  Symbols that do not contain the template type name are left
// unchanged (e.g. "New", "Wrap", "Register").
//
// The naming convention is useful when multiple instantiations of the same
// template coexist in one package — without it, every instantiation would
// produce identically named symbols.  For single-instantiation cases, the
// simpler "func New" style works fine.
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
