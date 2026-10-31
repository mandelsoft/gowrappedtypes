// Package basic demonstrates a simple gowt instantiation:
//   - default "via" (base.New)
//   - no "using" clause (PACKAGE. removed, leaving bare identifiers)
//   - *TemplateName* prefix/suffix substitution
// --- begin directive ---
package basic

//go:generate go run github.com/mandelsoft/gowrappedtypes

// gowt: type Service wrapping github.com/mandelsoft/gowrappedtypes/examples/template.Wrapper[*github.com/mandelsoft/gowrappedtypes/examples/base.Handler]

// --- end directive ---
