//go:build ignore

package testpkg

import "gowt.registry"

// Template[T] embeds T and uses both PACKAGE. and a gowt.registry placeholder.
type Template[T any] struct {
	T
	Extras PACKAGE.Extras
	Reg    *registry.Registry
}

// NewTemplate creates a Template using both placeholder packages.
func NewTemplate[T any](name string) (*Template[T], error) {
	v, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extras: *PACKAGE.NewExtras(name), Reg: registry.NewRegistry(name)}, nil
}
