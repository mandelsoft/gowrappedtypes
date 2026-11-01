// Package placeholder demonstrates gowt.* placeholder imports:
//   - template uses import "gowt.registry" instead of PACKAGE.
//   - directive maps the placeholder via a named "using" entry
//   - pointer base type (*base.Handler)
//   - default "via" (base.New)
// --- begin directive ---
package placeholder

//go:generate go run github.com/mandelsoft/gowrappedtypes

// gowt: type Gateway wrapping github.com/mandelsoft/gowrappedtypes/examples/template.Layered[*github.com/mandelsoft/gowrappedtypes/examples/base.Handler] using registry=github.com/mandelsoft/gowrappedtypes/examples/registry

// --- end directive ---
