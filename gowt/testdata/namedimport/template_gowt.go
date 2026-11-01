//go:build ignore

package testpkg

import "gowt.registry"

type Template[T any] struct {
	T
	Reg *registry.Registry
}

func NewTemplate[T any](name string) (*Template[T], error) {
	v, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Reg: registry.NewRegistry(name)}, nil
}
