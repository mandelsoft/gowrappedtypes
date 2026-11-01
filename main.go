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
// The template defines a struct type with one or more type parameters.  The
// first type parameter represents the embedded base type; additional type
// parameters are substituted with concrete types from the directive.  The
// template may also contain any number of functions (generic or non-generic)
// that operate on that type.
//
// Three special identifiers are recognised anywhere in a template:
//
//   - NEW is replaced by the constructor named in the "via" clause of the
//     instantiation directive (defaults to BasePkg.New when "via" is omitted).
//   - *TemplateName* — any symbol containing the template type name gets that
//     portion replaced by the generated type name, preserving both prefix and
//     suffix; see "Name conventions" below.
//   - PACKAGE. (always written with a trailing dot) is replaced by the alias
//     of the package named in the "using" clause, or removed entirely when no
//     "using" clause is present; see "Substitution pipeline" step 7.
//
// # Placeholder imports (gowt.*)
//
// When a template needs to reference multiple external packages that vary per
// instantiation, it can declare placeholder imports using the gowt. prefix:
//
//	import "gowt.registry"
//	import "gowt.config"
//
// These are not real Go imports.  The gowt. prefix marks them as placeholders
// that are resolved at generation time via named entries in the "using" clause.
// In the template code, the placeholder's alias (derived from the import path
// as usual, e.g. "registry" from "gowt.registry") is used as a normal Go
// package qualifier.  An explicit import alias is also supported:
//
//	import reg "gowt.registry"
//
// The directive maps each placeholder to a real package with name=pkg syntax
// in the "using" clause (see "Instantiation directives").
//
// Example template (wrapper_gowt.go):
//
//	package template
//
//	type Wrapper[Base any] struct {
//	    Base
//	    Registry *PACKAGE.Registry
//	    Label    string
//	}
//
//	func NewWrapper[Base any](name string) (*Wrapper[Base], error) {
//	    b, err := NEW(name)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return &Wrapper[Base]{Base: b, Registry: PACKAGE.NewRegistry(), Label: name}, nil
//	}
//
//	func (w *Wrapper[Base]) GetLabel() string { return w.Label }
//
//	func RegisterWrapper(reg *PACKAGE.Registry, w *Wrapper) { reg.Register(w.Label, w) }
//
//	func Describe(w *Wrapper) string { return "wrapper:" + w.Label }
//
// # Instantiation directives
//
// Instantiation is triggered by a specially formatted comment in any .go file
// in the target package:
//
//	// gowt: type Service wrapping [pkg.]Wrapper[BasePkg.BaseType]
//
// The "via" clause is optional; when omitted, NEW defaults to BasePkg.New.
// An unqualified function name uses the base type's package:
//
//	// gowt: type Service wrapping Wrapper[*base.Handler]
//	// gowt: type Service wrapping Wrapper[*base.Handler via New]
//	// gowt: type Service wrapping Wrapper[*base.Handler via base.New]
//
// When the template has multiple type parameters, additional concrete types
// are listed after the "via" clause (or after the base type), separated by
// commas.  They are matched to the template's type parameters by position:
//
//	// gowt: type Service wrapping Wrapper[*base.Handler via New, cfg.Config]
//	// gowt: type Service wrapping Wrapper[*base.Handler, cfg.Config]
//
// An optional "using" clause names packages for placeholder substitution.
// A bare package maps to the PACKAGE. placeholder; named entries (name=pkg)
// map to gowt.<name> placeholder imports in the template.  Entries are
// comma-separated.  When the default "using" package is omitted, PACKAGE.
// is removed, leaving an unqualified identifier suitable for the local package:
//
//	// gowt: type Endpoint wrapping Wrapper[*base.Handler] using registry
//	// gowt: type Endpoint wrapping Wrapper[*base.Handler] using registry=github.com/example/registry
//	// gowt: type Endpoint wrapping Wrapper[*base.Handler] using github.com/example/extras, registry=github.com/example/registry
//
// An optional "renaming" clause maps identifiers to new names in the generated
// output:
//
//	// gowt: type Endpoint wrapping Wrapper[*base.Handler] renaming Describe=Info
//
// Multiple renames are comma-separated.  Both clauses may be combined, with
// "using" preceding "renaming":
//
//	// gowt: type Endpoint wrapping Wrapper[*base.Handler via base.Create] using registry renaming Describe=Info
//
// The template may live in a different package; use its import path or a local
// alias as the package qualifier:
//
//	// gowt: type Service wrapping mypkg.Wrapper[*base.Handler]
//
// # Substitution pipeline
//
// For each declaration in the template the following substitutions are applied
// in order.  Steps 1-2 run pre-order (parent before children); steps 0 and
// 3-7 run post-order (children before parent).  Step 0 is numbered first
// because explicit renames take the highest priority.
//
//  0. Explicit renames (from the "renaming" clause): OldName → NewName for
//     every matching identifier (post-order).
//  1. [prefix]*TemplateName*[suffix][...] → [prefix]*GeneratedName*[suffix]
//     (type expressions with type-argument lists; prefix and suffix around
//     the template name are preserved; pre-order).
//  2. func F[TypeParam ...] → func F  (removes type parameters from function
//     signatures; pre-order).
//  3. TypeParam: → ConcreteType:  (struct-literal field keys; applies to all
//     type parameters).
//  4. TypeParam → [*]Pkg.ConcreteType  (all remaining occurrences in type
//     expressions; the "*" prefix is added when the type argument is a
//     pointer; applies to all type parameters).
//  5. Bare *TemplateName* → *GeneratedName*  (all identifiers containing the
//     template name, with the same prefix/suffix preservation as step 1;
//     qualified references like pkg.Name are left untouched).
//  6. NEW → CtorPkg.CtorFunc.
//  7. PACKAGE. → usingAlias. (named package) or "" (local package — no qualifier needed in Go).
//
// # Name conventions
//
// Any symbol whose name contains the template type name gets that portion
// replaced by the generated type name, preserving both prefix and suffix.
// For example, with template type "Wrapper" and generated type "Service":
//
//	Wrapper          → Service          (exact match)
//	NewWrapper       → NewService       (prefix preserved)
//	WrapperLabel     → ServiceLabel     (suffix preserved)
//	RegisterWrapper  → RegisterService  (prefix preserved)
//
// This applies equally to generic functions, non-generic functions, and
// methods — for generic functions and methods, the type parameter list is
// stripped first, then the name is substituted.  Symbols that do not contain
// the template type name are left unchanged (e.g. "Describe", "GetLabel").
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

//go:generate go run github.com/mandelsoft/mdref doc/src .

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
