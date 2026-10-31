// Package full demonstrates an advanced gowt instantiation:
//   - pointer base type (*base.Handler)
//   - explicit "via" constructor (base.Create)
//   - "using" clause (PACKAGE. → registry.)
//   - "renaming" clause (Describe=Info)
//   - cross-package template reference
//   - *TemplateName* prefix/suffix substitution
// --- begin directive ---
package full

//go:generate go run github.com/mandelsoft/gowrappedtypes

// gowt: type Endpoint wrapping github.com/mandelsoft/gowrappedtypes/examples/template.Wrapper[*github.com/mandelsoft/gowrappedtypes/examples/base.Handler via github.com/mandelsoft/gowrappedtypes/examples/base.Create] using github.com/mandelsoft/gowrappedtypes/examples/registry renaming Describe=Info

// --- end directive ---
