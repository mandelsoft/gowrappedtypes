# gowt - Go Wrapper Types

`gowt` is a code generator that produces concrete Go types from parameterized
`_gowt.go` template files.  It bridges the gap between Go's type-parameter syntax
and the common pattern of *wrapping* one environment/base type inside another
with additional fields and behaviour.  Rather than hand-writing repetitive
struct+constructor combinations, you write a single generic template once and
instantiate it for every concrete base type with a one-line comment directive.

## Motivation

Go generics let you write parameterized code, but they cannot remove the type
parameter from the public API: callers must always spell out the full
`MyType[ConcreteBase]`.  When building layered environment or decorator
hierarchies, you often want a *concrete* exported type that embeds a specific
base, with its own constructor and helpers — but the struct layout and
constructor body are structurally identical across all layers.

`gowt` automates exactly this: you author a `_gowt.go` template in valid Go
generics syntax, then stamp out concrete, non-generic types for each base you
need.  Template files carry a `//go:build ignore` constraint so the Go toolchain
excludes them from normal builds but still includes them in module downloads —
this means cross-package templates work even when the template lives in a
dependency fetched from the module proxy.  The generated code is fully
`gofmt`-formatted, carries a `DO NOT EDIT` header, and is meant to be committed
alongside your sources.

## Quick start

1. Write a `_gowt.go` template (see [Template files](#template-files)).
2. Add an instantiation directive comment to any `.go` file in the target
   package (see [Instantiation directives](#instantiation-directives)).
3. Run the generator:

```bash
# Explicitly:
go run github.com/mandelsoft/gowrappedtypes/cmd/gowt env.go

# Or via go:generate (place this in the .go file with the directive):
//go:generate go run github.com/mandelsoft/gowrappedtypes/cmd/gowt
```

The generated file is written to `<lowercase(TypeName)>_gowt.go` in the same
directory as the `.go` file containing the directive.

## Template files

A `_gowt.go` file is ordinary Go source — parseable by `go/parser` — that
defines a **struct type with exactly one type parameter** representing the
embedded base type, together with any number of functions (generic or
non-generic) that operate on that type.  Every template must start with
`//go:build ignore` so the Go toolchain skips it during normal builds.

Three special identifiers are recognised anywhere in a template:

| Placeholder | Meaning |
|---|---|
| `NEW`        | Replaced by the constructor named in the `via` clause of the instantiation directive. |
| `PACKAGE.`   | Replaced by the alias of the package named in the `using` clause, or removed entirely when no `using` clause is present (leaving a bare, unqualified identifier suitable for the local package). |
| *TemplateName* | The template type name itself (and any camelCase-prefixed form of it) is replaced by the generated type name.  See [Name conventions](#name-conventions). |

> **Note:** `PACKAGE.NEW` is not valid.  `NEW` refers to the constructor
> specified by the `via` clause, while `PACKAGE.` refers to the package
> specified by the `using` clause.  These are independent concerns and must be
> used separately.  The generator reports an error if `PACKAGE.NEW` appears in a
> template.

### Example template

```go
//go:build ignore

// environment_gowt.go
package composer

import "github.com/mandelsoft/composer/epi"

type Environment[Base epi.Environment] struct {
    Base
    ConfigSetGroup
}

func New[Base epi.Environment](opts ...epi.Option) (*Environment[Base], error) {
    e := epi.NewEnvState(opts...)
    base, err := NEW(e)
    if err != nil {
        return nil, err
    }
    env := &Environment[Base]{
        Base:           base,
        ConfigSetGroup: *PACKAGE.NewGroup(e),
    }
    return epi.WithOptionsApplied(env, opts...)
}
```

### What can go in a template

A template may contain any combination of:

- **One parameterized struct type** — the "main" declaration that drives the
  substitution.  Its single type parameter is the base type that will be
  replaced during instantiation.

- **Generic functions** — functions with the same type parameter as the struct.
  The type parameter list is stripped from the generated output, and the type
  parameter name is replaced by the concrete base type.

- **Non-generic functions** — plain functions that reference the template type
  name in their signature or body.  The type name references are substituted,
  but no type parameters are involved.  This is useful for helpers, validators,
  or registration functions that operate on the concrete type.

- **Doc comments on declarations** — these are stripped from the generated
  output (they may reference placeholders or the generic syntax that would be
  confusing in the concrete code).  Inline comments within function bodies are
  preserved.

## Instantiation directives

Instantiation is triggered by a specially formatted comment in any `.go` file in
the target package.  The basic form is:

```go
// gowt: type GeneratedName wrapping TemplateName[BasePkg.BaseType via CtorPkg.CtorFunc]
```

This tells `gowt` to:
1. Find a `_gowt.go` file containing a type called `TemplateName`.
2. Replace the type parameter with `BasePkg.BaseType`.
3. Replace `NEW` with `CtorPkg.CtorFunc`.
4. Emit the result as a concrete type named `GeneratedName`.

### Directive syntax

The full syntax, with all optional clauses, is:

```
// gowt: type <Name> wrapping [<TmplPkg>.]<Template>[<BaseType> via <Ctor>] [using <Pkg>] [renaming <Old>=<New>,...]
```

Each part is described below.

### Base type

The base type is written as `Pkg.Type` inside the brackets.  It may be a
pointer type:

```go
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New]
```

When the base type is a pointer, the generated struct embeds `*Pkg.Type` (with
the star), but struct-literal field keys still use the unqualified type name
without the star (as Go requires).

Package references can be either **short aliases** (resolved from the imports of
the `.go` file containing the directive) or **full import paths**:

```go
// Short alias — "common" is looked up in the file's imports:
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New]

// Full import path — resolved directly, last path element becomes the alias:
// gowt: type MyEnv wrapping Environment[*github.com/mandelsoft/composer/common.Environment via github.com/mandelsoft/composer/common.New]
```

### Constructor (`via`)

The `via` clause names the constructor function that `NEW` is replaced with.  It
must be a qualified name (`Pkg.Func`):

```go
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New]
```

In the generated code, every occurrence of `NEW` in the template becomes
`common.New`.

### Cross-package templates

The template may live in a different package.  Prefix the template name with the
package path or alias:

```go
// gowt: type MyEnv wrapping mypkg.Environment[*common.Environment via common.New]
```

The generator uses `go list` to resolve the package to a directory and searches
for `_gowt.go` files there.

### The `using` clause

The optional `using` clause names a package whose import is added to the
generated file and whose alias replaces the `PACKAGE.` placeholder throughout
the template:

```go
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] using github.com/example/extras
```

In the template, `PACKAGE.Extras` becomes `extras.Extras`, and
`PACKAGE.NewExtras(x)` becomes `extras.NewExtras(x)`.

When `using` is **omitted**, `PACKAGE.` is removed entirely, leaving bare
unqualified identifiers.  This is the correct behaviour when the referenced
types and functions are in the same package as the generated code:

| Template source | With `using extras` | Without `using` |
|---|---|---|
| `PACKAGE.Extras`      | `extras.Extras` | `Extras` |
| `PACKAGE.NewExtras(x)` | `extras.NewExtras(x)` | `NewExtras(x)` |

### The `renaming` clause

The optional `renaming` clause lets you rename specific identifiers in the
generated output.  This is primarily useful for function names when the template
uses a generic name like `New` but you need a distinct name in the target
package (e.g., because multiple instantiations coexist):

```go
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] renaming New=NewMyEnv
```

Multiple renames are comma-separated:

```go
// gowt: ... renaming New=NewMyEnv,Wrap=BuildMyEnv
```

Renames apply to all word-boundary occurrences of the old name — both the
declaration itself and any call sites within the template body.

Both `using` and `renaming` can be combined.  When present, `using` must come
first:

```go
// gowt: type MyEnv wrapping Environment[*common.Environment via common.New] using mypkg renaming New=NewMyEnv
```

## Name conventions

When a function name in the template ends with the template type name, the
generator preserves the prefix and appends the generated type name instead.
This is the **camelCase-prefix convention**:

| Template function | Template type | Generated type | Generated function |
|---|---|---|---|
| `NewEnvironment` | `Environment` | `MyEnv` | `NewMyEnv` |
| `DefaultEnvironment` | `Environment` | `MyEnv` | `DefaultMyEnv` |
| `WrapEnvironment` | `Environment` | `MyEnv` | `WrapMyEnv` |

This convention is entirely optional.  If a function name does not end with the
template type name, it is left unchanged (unless it matches an explicit rename):

| Template function | Template type | Generated function |
|---|---|---|
| `New`      | `Environment` | `New` (unchanged) |
| `Wrap`     | `Environment` | `Wrap` (unchanged) |
| `Register` | `Environment` | `Register` (unchanged) |

The camelCase-prefix convention is useful when multiple instantiations of the
same template coexist in one package — without it, every instantiation would
produce identically named functions, causing compile errors.  For
single-instantiation cases, the simpler `func New` style works fine.

## Substitution pipeline

For each declaration in the template, the following substitutions are applied in
order.  Steps 0-2 use **pre-order** AST traversal (parent before children);
steps 3-7 use **post-order** (children before parent).

| Step | Pattern | Replacement | Notes |
|---|---|---|---|
| 0 | `OldName` (from `renaming`) | `NewName` | Highest priority; applies everywhere. |
| 1 | `[prefix]TemplateName[...]` | `[prefix]GeneratedName` | Type expressions with type-argument lists.  The prefix is preserved (camelCase convention). |
| 2 | `func F[TypeParam ...]` | `func F` | Strips type parameters from function and type declarations. |
| 3 | `TypeParam:` (in struct literal key) | `BaseType:` | Unqualified base type name only (Go struct literal keys must not be pointer-qualified). |
| 4 | `TypeParam` (elsewhere) | `[*]BasePkg.BaseType` | Full qualified type; `*` added when the base type is a pointer. |
| 5 | bare `TemplateName` | `GeneratedName` | References without type-argument lists.  camelCase prefix preserved.  Idents in `Sel` position of a `SelectorExpr` (e.g., `epi.Environment`) are left untouched. |
| 6 | `NEW` | `CtorPkg.CtorFunc` | Constructor placeholder. |
| 7 | `PACKAGE.X` | `UsingAlias.X` or `X` | Package placeholder.  When no `using` clause, the qualifier is removed. |

## Output

Each instantiation directive produces one file:

```
<lowercase(GeneratedName)>_gowt.go
```

in the same directory as the `.go` file containing the directive.  The file
begins with:

```go
// Code generated by gowt from <template>_gowt.go. DO NOT EDIT.
```

The output is always `gofmt`-formatted.  Imports from the template file are
carried over to the generated file, and additional imports are added as needed
for the base type, constructor, using package, and cross-package template
references.  Duplicate imports are deduplicated; stdlib imports are sorted before
third-party imports.

## Complete example

**Template** (`environment_gowt.go`):
```go
//go:build ignore

package composer

import "github.com/mandelsoft/composer/epi"

type Environment[Base epi.Environment] struct {
    Base
    ConfigSetGroup
}

func New[Base epi.Environment](opts ...epi.Option) (*Environment[Base], error) {
    e := epi.NewEnvState(opts...)
    base, err := NEW(e)
    if err != nil {
        return nil, err
    }
    env := &Environment[Base]{
        Base:           base,
        ConfigSetGroup: *PACKAGE.NewGroup(e),
    }
    return epi.WithOptionsApplied(env, opts...)
}
```

**Directive** (`env.go`):
```go
package composer

//go:generate go run github.com/mandelsoft/gowrappedtypes/cmd/gowt

// gowt: type Environment wrapping Environment[*github.com/mandelsoft/composer/common.Environment via github.com/mandelsoft/composer/common.New]
```

**Generated output** (`environment_gowt.go`):
```go
// Code generated by gowt from environment_gowt.go. DO NOT EDIT.

package composer

import (
    "github.com/mandelsoft/composer/common"
    "github.com/mandelsoft/composer/epi"
)

type Environment struct {
    *common.Environment
    ConfigSetGroup
}

func New(opts ...epi.Option) (*Environment, error) {
    e := epi.NewEnvState(opts...)
    base, err := common.New(e)
    if err != nil {
        return nil, err
    }
    env := &Environment{
        Environment:    base,
        ConfigSetGroup: *NewGroup(e),
    }
    return epi.WithOptionsApplied(env, opts...)
}
```

Note how every placeholder was resolved:
- The type parameter `Base` became `*common.Environment` (pointer base type).
- `Environment[Base]` in type expressions became the bare `Environment`.
- `Base:` in the struct literal became `Environment:` (the unqualified base type name).
- `NEW(e)` became `common.New(e)`.
- `PACKAGE.NewGroup(e)` became `NewGroup(e)` (no `using` clause, so the qualifier was removed — `NewGroup` lives in the local package).
