# gowt - Go Wrapper Types

> This project — code, tests, and documentation — was entirely developed by
> [Claude Code](https://claude.ai/claude-code) (Anthropic's AI coding agent).

`gowt` is a code generator that produces concrete Go types from parameterized
`_gowt.go` template files.  It bridges the gap between Go's type-parameter syntax
and the common pattern of embedding a parametric base type in a struct with
additional fields and behaviour.  Rather than hand-writing repetitive
struct+constructor combinations, you write a single generic template once and
instantiate it for every concrete base type with a one-line comment directive.

## Motivation

A common Go pattern is a struct that *embeds* a base type and adds fields or
behaviour on top — decorators, layered configurations, middleware wrappers, etc.
When the same struct layout needs to work with several different base types, the
natural approach would be a generic struct that embeds its type parameter:

```go
type Wrapper[Base any] struct {
    Base        // ← embed the type parameter
    ExtraField  int
}
```

However, **Go does not allow embedding a type parameter** — the compiler
rejects `Base` as an embedded field when `Base` is a type parameter.  This
means you cannot write a single generic type that embeds different base types;
instead you must hand-write a separate struct (plus constructor, helpers, etc.)
for every concrete base — even though the struct layout and constructor body
are structurally identical.

Go generics have a second limitation here: even if embedding were allowed,
callers would always have to spell out the full `Wrapper[ConcreteBase]`.  In a
public API you typically want a plain, concrete type name.

`gowt` solves both problems.  You author a `_gowt.go` template in valid Go
generics syntax (with a `//go:build ignore` constraint so the compiler never
sees the illegal embedding), then stamp out concrete, non-generic types for
each base you need with a one-line comment directive.  The template files use
`.go` extension so they are included in module downloads — this means
cross-package templates work even when the template lives in a dependency
fetched from the module proxy.  The generated code is fully `gofmt`-formatted,
carries a `DO NOT EDIT` header, and is meant to be committed alongside your
sources.

## Quick start

1. Write a `_gowt.go` template (see [Template files](#template-files)).
2. Add an instantiation directive comment to any `.go` file in the target
   package (see [Instantiation directives](#instantiation-directives)).
3. Run the generator:

```bash
# Explicitly:
go run github.com/mandelsoft/gowrappedtypes types.go

# Or via go:generate (place this in the .go file with the directive):
//go:generate go run github.com/mandelsoft/gowrappedtypes
```

The generated file is written to `<lowercase(TypeName)>_gowtgen.go` in the same
directory as the `.go` file containing the directive.

## Template files

A `_gowt.go` file is ordinary Go source — parseable by `go/parser` — that
defines a **struct type with one or more type parameters**.  The first type
parameter represents the embedded base type; additional type parameters are
substituted with concrete types from the directive.  The first type parameter
is used as an embedded field, which is not valid Go — the `//go:build ignore`
constraint ensures the compiler never sees the template, while `go/parser`
(which does not type-check) can still parse it.

### Placeholders

Three special identifiers are recognised anywhere in a template:

| Placeholder | Meaning |
|---|---|
| `NEW`        | Replaced by the constructor from the `via` clause (or `BasePkg.New` by default). |
| `PACKAGE.`   | Replaced by the alias of the package named in the `using` clause, or removed entirely when no `using` clause is present (leaving a bare, unqualified identifier suitable for the local package). |
| `*TemplateName*` | Any symbol containing the template type name gets that portion replaced by the generated type name, preserving both prefix and suffix.  The `*` denotes arbitrary characters.  Applies to top-level symbols (types, functions, variables, constants).  See [Name conventions](#name-conventions). |

> **Note:** `PACKAGE.NEW` is not valid.  `NEW` refers to the constructor
> specified by the `via` clause, while `PACKAGE.` refers to the package
> specified by the `using` clause.  These are independent concerns and must be
> used separately.  The generator reports an error if `PACKAGE.NEW` appears in a
> template.

### Example template

(`examples/template/wrapper_gowt.go`):

```go
{{include}{../../examples/template/wrapper_gowt.go}}
```

### What can go in a template

A template may contain any combination of:

- **One parameterized struct type** — the "main" declaration that drives the
  substitution.  Its first type parameter is the embedded base type; additional
  type parameters (if any) are replaced with the concrete types given in the
  directive.

- **Generic functions** — functions with the same type parameter as the struct.
  The type parameter list is stripped from the generated output, and the type
  parameter name is replaced by the concrete base type.

- **Methods on the parameterized type** — methods with a receiver of the
  template type (e.g. `func (w *Wrapper[Base]) GetLabel() string`).  The
  receiver type is substituted to the generated type; the type parameter list
  is stripped.

- **Non-generic functions** — plain functions that reference the template type
  name in their signature or body.  The type name references are substituted,
  but no type parameters are involved.  This is useful for helpers, validators,
  or registration functions that operate on the concrete type.  All
  placeholders (`NEW`, `PACKAGE.`, `*TemplateName*`) work identically in
  non-generic functions — including in parameter types, return types, and
  function bodies.

```go
{{include}{../../examples/template/wrapper_gowt.go}{non-generic functions}}
```

- **Doc comments on declarations** — these are stripped from the generated
  output (they may reference placeholders or the generic syntax that would be
  confusing in the concrete code).  Inline comments within function bodies are
  preserved.

## Instantiation directives

Instantiation is triggered by a specially formatted comment in any `.go` file in
the target package.  The simplest form is:

```go
// gowt: type Service wrapping Wrapper[*base.Handler]
```

This tells `gowt` to:
1. Find a `_gowt.go` file containing a type called `Wrapper`.
2. Replace the type parameter with `*base.Handler`.
3. Replace `NEW` with `base.New` (the default constructor).
4. Emit the result as a concrete type named `Service`.

### Directive syntax

The full syntax, with all optional clauses, is:

```
// gowt: type <Name> wrapping [<TmplPkg>.]<Template>[<BaseType> [via [<CtorPkg>.]<Ctor>][, <Type2>[, <Type3>...]]] [using <Pkg>] [renaming <Old>=<New>,...]
```

Each part is described below.

### Multiple type parameters

When the template has more than one type parameter, the additional concrete
types are listed after the `via` clause (or after the base type when `via` is
omitted), separated by commas:

```go
// Template with two type parameters:
// type Wrapper[Base any, Config fmt.Stringer] struct { ... }

// Directive — base type + extra type arg:
// gowt: type Service wrapping Wrapper[*base.Handler via base.New, cfg.Config]

// Without via:
// gowt: type Service wrapping Wrapper[*base.Handler, cfg.Config]
```

The extra type arguments are matched to the template's type parameters by
position.  The number of type arguments in the directive must match the number
of type parameters in the template.  Each extra argument follows the same
`[*]Pkg.Type` syntax as the base type.

When any type parameter has a non-trivial constraint (anything other than
`any`), the generated file includes a compile-time assertion that verifies all
type arguments satisfy their respective constraints.

### Base type

The base type is written as `Pkg.Type` inside the brackets.  It may be a
pointer type (`*Pkg.Type`).

When the base type is a pointer, the generated struct embeds `*Pkg.Type` (with
the star), but struct-literal field keys still use the unqualified type name
without the star (as Go requires).

Package references can be either **short aliases** (resolved from the imports of
the `.go` file containing the directive) or **full import paths**:

```go
// Short alias — "base" is looked up in the file's imports:
// gowt: type Service wrapping Wrapper[*base.Handler]

// Full import path — resolved directly, last path element becomes the alias:
// gowt: type Service wrapping Wrapper[*github.com/example/base.Handler]
```

### Constructor (`via`)

The optional `via` clause names the constructor function that `NEW` is replaced
with.  It supports three forms:

| Directive | `NEW` resolves to | Notes |
|---|---|---|
| `Wrapper[*base.Handler]` | `base.New` | Default: `New` in the base type's package |
| `Wrapper[*base.Handler via Create]` | `base.Create` | Unqualified: function in the base type's package |
| `Wrapper[*base.Handler via other.Create]` | `other.Create` | Fully qualified: explicit package |

When `via` is omitted, the constructor defaults to `New` in the base type's
package.  When `via` names an unqualified function (no dot), the base type's
package is used.  A fully qualified name (`Pkg.Func`) may reference a different
package than the base type.

### Cross-package templates

The template may live in a different package.  Prefix the template name with the
package path or alias:

```go
{{include}{../../examples/basic/types.go}{directive}}
```

The generator uses `go list` to resolve the package to a directory and searches
for `_gowt.go` files there.

### The `using` clause

The optional `using` clause names packages for placeholder substitution.
Entries are comma-separated.  A bare package maps to the `PACKAGE.` placeholder;
named entries (`name=pkg`) map to `gowt.<name>` placeholder imports in the
template:

```go
// Default PACKAGE. mapping (backward compatible):
// gowt: type Endpoint wrapping Wrapper[*base.Handler] using github.com/example/registry

// Named placeholder mapping:
// gowt: type Endpoint wrapping Wrapper[*base.Handler] using registry=github.com/example/registry

// Both default and named:
// gowt: type Endpoint wrapping Wrapper[*base.Handler] using github.com/example/extras, registry=github.com/example/registry
```

| Template source | With `using registry` | Without `using` |
|---|---|---|
| `PACKAGE.Registry`       | `registry.Registry`       | `Registry` |
| `PACKAGE.NewRegistry()`  | `registry.NewRegistry()`  | `NewRegistry()` |
| `PACKAGE.Registry` (parameter type) | `registry.Registry` | `Registry` |

When `using` is **omitted**, `PACKAGE.` is removed entirely, leaving bare
unqualified identifiers.  This is the correct behaviour when the referenced
types and functions are in the same package as the generated code.

### Placeholder imports (`gowt.*`)

When a template needs to reference multiple external packages that vary per
instantiation, it can declare **placeholder imports** using the `gowt.` prefix:

```go
import "gowt.registry"
import "gowt.config"
```

These are not real Go imports — `go/parser` can parse them, but the `gowt.`
prefix marks them as placeholders resolved at generation time.  In the template
code, the placeholder's alias (derived from the import path as usual, e.g.
`registry` from `gowt.registry`) is used as a normal Go package qualifier:

```go
//go:build ignore

package mypkg

import "gowt.registry"

type Template[Base any] struct {
    Base
    Reg *registry.Registry
}

func NewTemplate[Base any](name string) (*Template[Base], error) {
    v, err := NEW(name)
    if err != nil {
        return nil, err
    }
    return &Template[Base]{Base: v, Reg: registry.NewRegistry(name)}, nil
}
```

The directive maps each placeholder to a real package using `name=pkg` entries
in the `using` clause:

```go
// gowt: type Service wrapping Template[*base.Handler] using registry=github.com/example/registry
```

An explicit import alias is also supported in the template:

```go
import reg "gowt.registry"
// Template code uses reg.Registry, reg.NewRegistry, etc.
```

All `gowt.*` placeholder imports in the template **must** have a corresponding
`using` entry; the generator reports an error if a mapping is missing.

The `gowt.*` mechanism and `PACKAGE.` can be combined in the same template.
`PACKAGE.` is mapped by the bare (unnamed) `using` entry, while `gowt.*`
imports are mapped by named entries:

```go
// gowt: type Service wrapping Template[*base.Handler] using github.com/example/extras, registry=github.com/example/registry
```

### The `renaming` clause

The optional `renaming` clause lets you rename specific identifiers in the
generated output.  This is primarily useful for function names when the template
uses a generic name like `Describe` but you need a distinct name in the target
package (e.g., because multiple instantiations coexist):

```go
// gowt: type Endpoint wrapping Wrapper[*base.Handler] renaming Describe=Info
```

Multiple renames are comma-separated:

```go
// gowt: ... renaming Describe=Info,RegisterWrapper=AddEndpoint
```

Renames apply to all matching identifiers — both the declaration itself and any
call sites within the template body.

> **When is `renaming` needed?**  The `*TemplateName*` convention only renames
> symbols that *contain* the template type name.  A function called simply
> `Describe` does not contain `Wrapper`, so it stays `Describe` after
> instantiation.  When two instantiations coexist in the same package, both
> would produce a `func Describe` — use `renaming Describe=Info` to
> disambiguate.

Both `using` and `renaming` can be combined.  When present, `using` must come
first:

```go
// gowt: type Endpoint wrapping Wrapper[*base.Handler] using registry renaming Describe=Info
```

### Directive examples at a glance

| Directive | Effect |
|---|---|
| `Wrapper[*base.Handler]` | Simplest form; constructor defaults to `base.New` |
| `Wrapper[*base.Handler via Create]` | Custom constructor `base.Create` |
| `Wrapper[*base.Handler via other.Create]` | Constructor from a different package |
| `Wrapper[*base.Handler] using registry` | `PACKAGE.X` becomes `registry.X` |
| `Wrapper[*base.Handler] renaming Describe=Info` | Rename `Describe` to `Info` in output |
| `Wrapper[*base.Handler] using registry renaming Describe=Info` | Combined clauses |
| `mypkg.Wrapper[*base.Handler]` | Template from another package |
| `Wrapper[*base.Handler, cfg.Config]` | Extra type parameter (matched by position) |
| `Wrapper[*base.Handler via New, cfg.Config]` | Extra type parameter with explicit `via` |
| `Template[*base.Handler] using reg=github.com/example/registry` | `gowt.reg` placeholder resolved to real package |
| `Template[*base.Handler] using github.com/example/extras, reg=github.com/example/registry` | `PACKAGE.` and `gowt.reg` combined |

## Name conventions

The `*TemplateName*` placeholder applies to all identifiers in the template.  Any identifier whose name
contains the template type name gets that portion replaced by the generated
type name, preserving both prefix and suffix:

| Template symbol | Template type | Generated type | Generated symbol |
|---|---|---|---|
| `Wrapper` | `Wrapper` | `Service` | `Service` |
| `NewWrapper` | `Wrapper` | `Service` | `NewService` |
| `WrapperLabel` | `Wrapper` | `Service` | `ServiceLabel` |
| `RegisterWrapper` | `Wrapper` | `Service` | `RegisterService` |

The substitution applies equally to generic and non-generic functions, and to
methods.  For generic functions and methods, the type parameter list is stripped
first (step 2), then the name is substituted (step 5):

```go
// Template:
func (w *Wrapper[Base]) GetLabel() string { return w.Label }

// Generated (Wrapper→Service):
func (w *Service) GetLabel() string { return w.Label }
```

Symbols that do **not** contain the template type name are left unchanged
(unless they match an explicit rename):

| Template symbol | Template type | Generated symbol |
|---|---|---|
| `Describe`  | `Wrapper` | `Describe` (unchanged) |
| `GetLabel`  | `Wrapper` | `GetLabel` (unchanged) |

Qualified references (the right-hand side of `pkg.Name`, e.g. `base.Handler`)
are never renamed, even if they happen to match the template type name.

The naming convention is useful when multiple instantiations of the same
template coexist in one package — without it, every instantiation would produce
identically named symbols, causing compile errors.  For single-instantiation
cases, the simpler `func New` style works fine.

## Substitution pipeline

For each declaration in the template, the following substitutions are applied in
order.  Steps 1-2 use **pre-order** AST traversal (parent before children);
steps 0 and 3-7 use **post-order** (children before parent).  Step 0 is
numbered first because explicit renames take the highest priority when an
identifier matches both a rename and another rule.

| Step | Pattern | Replacement | Notes |
|---|---|---|---|
| 0 | `OldName` (from `renaming`) | `NewName` | Highest priority; applies everywhere (post-order). |
| 1 | `*TemplateName*[...]` | `*GeneratedName*` | Type expressions with type-argument lists.  Prefix and suffix are preserved (pre-order). |
| 2 | `func F[TypeParam ...]` | `func F` | Strips type parameters from function and type declarations (pre-order). |
| 3 | `TypeParam:` (in struct literal key) | `ConcreteType:` | Unqualified type name only (Go struct literal keys must not be pointer-qualified).  Applies to all type parameters. |
| 4 | `TypeParam` (elsewhere) | `[*]Pkg.ConcreteType` | Full qualified type; `*` added when the type argument is a pointer.  Applies to all type parameters. |
| 5 | bare `*TemplateName*` | `*GeneratedName*` | All identifiers containing the template type name.  Prefix and suffix preserved.  Qualified references (`pkg.Name`) are left untouched. |
| 6 | `NEW` | `CtorPkg.CtorFunc` | Constructor placeholder. |
| 7 | `PACKAGE.X` | `UsingAlias.X` or `X` | Package placeholder.  When no `using` clause, the qualifier is removed. |

## Output

Each instantiation directive produces one file:

```
<lowercase(GeneratedName)>_gowtgen.go
```

in the same directory as the `.go` file containing the directive.  The file
begins with:

```go
// Code generated by gowt from <template>_gowt.go. DO NOT EDIT.
```

The output is always `gofmt`-formatted.  Imports from the template file are
carried over to the generated file, and additional imports are added as needed
for the base type, constructor, and using package.  Duplicate imports are
deduplicated; stdlib imports are sorted before third-party imports.

## Complete examples

A working example tree is provided under [`examples/`](examples/).  The
`wrapper_gowt.go` template is instantiated twice — once with minimal options
(basic) and once with all clauses (full).  A second template
(`layered_gowt.go`) demonstrates `gowt.*` placeholder imports:

```
examples/
├── base/                          # base type package
│   └── base.go                    # Handler type, New + Create constructors
├── registry/                      # "using" target package
│   └── registry.go                # Registry type, NewRegistry
├── template/                      # template package (cross-package)
│   ├── doc.go                     # package declaration for go list
│   ├── wrapper_gowt.go           # template: NEW, PACKAGE., *Wrapper*
│   └── layered_gowt.go           # template: gowt.registry placeholder import
├── basic/                         # simple instantiation
│   ├── types.go                   # directive: default via, no using
│   ├── registry.go                # local Registry (PACKAGE. stripped)
│   └── service_gowtgen.go         # generated
├── full/                          # advanced instantiation
│   ├── types.go                   # directive: via, using, renaming
│   └── endpoint_gowtgen.go        # generated
└── placeholder/                   # gowt.* placeholder import instantiation
    ├── types.go                   # directive: named using entry
    └── gateway_gowtgen.go         # generated
```

### Basic — default constructor, no `using`

**Directive** (`basic/types.go`):

```go
{{include}{../../examples/basic/types.go}{directive}}
```

**Generated output** (`basic/service_gowtgen.go`):

```go
{{include}{../../examples/basic/service_gowtgen.go}}
```

Note how every placeholder was resolved:
- The type parameter `Base` became `*base.Handler` (pointer base type).
- `Wrapper[Base]` in type expressions and receivers became the bare `Service`.
- `Base:` in the struct literal became `Handler:` (the unqualified base type name).
- `NEW(name)` became `base.New(name)` (no `via` clause — defaults to `New` in the base package).
- `PACKAGE.Registry` and `PACKAGE.NewRegistry()` became `Registry` and `NewRegistry()` (no `using` clause — qualifier removed, types live in the local package).
- `NewWrapper` became `NewService` and `RegisterWrapper` became `RegisterService` (`*TemplateName*` prefix substitution).
- `Describe` stayed `Describe` (does not contain `Wrapper`).
- `(w *Wrapper[Base]) GetLabel()` became `(w *Service) GetLabel()` (method receiver substituted).
- Doc comments were stripped; inline comments preserved.

### Full — explicit `via`, `using`, and `renaming`

**Directive** (`full/types.go`):

```go
{{include}{../../examples/full/types.go}{directive}}
```

**Generated output** (`full/endpoint_gowtgen.go`):

```go
{{include}{../../examples/full/endpoint_gowtgen.go}}
```

Compared to the basic case:
- `NEW(name)` became `base.Create(name)` (explicit `via base.Create`).
- `PACKAGE.Registry` became `registry.Registry` (`using registry` adds the import and qualifies all `PACKAGE.` references).
- `Describe` became `Info` (`renaming Describe=Info`).
- `NewWrapper` → `NewEndpoint` and `RegisterWrapper` → `RegisterEndpoint` (`*TemplateName*` substitution with target name `Endpoint`).

### Placeholder imports — `gowt.*` instead of `PACKAGE.`

The `layered_gowt.go` template uses `import "gowt.registry"` — a placeholder
import — instead of the `PACKAGE.` convention.  The template code uses
`registry.` as a normal Go qualifier:

**Template** (`template/layered_gowt.go`):

```go
{{include}{../../examples/template/layered_gowt.go}}
```

**Directive** (`placeholder/types.go`):

```go
{{include}{../../examples/placeholder/types.go}{directive}}
```

**Generated output** (`placeholder/gateway_gowtgen.go`):

```go
{{include}{../../examples/placeholder/gateway_gowtgen.go}}
```

The `gowt.registry` import was replaced by the real `registry` package.
Unlike `PACKAGE.`, placeholder imports work like regular Go imports — the
template code uses the alias as a normal qualifier, so editors and linters
can reason about the code even before generation.
