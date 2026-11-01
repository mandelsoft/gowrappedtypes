//go:build ignore

package testpkg

import "gowt.registry"

// Template[T] embeds T and uses a registry from a placeholder import.
type Template[T any] struct {
	T
	Reg *registry.Registry
}

// NewTemplate creates a Template with a Registry from the placeholder package.
func NewTemplate[T any](name string) (*Template[T], error) {
	v, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Reg: registry.NewRegistry(name)}, nil
}
