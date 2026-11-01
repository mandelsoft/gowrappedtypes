//go:build ignore

package testpkg

import reg "gowt.registry"

type Template[T any] struct {
	T
	Reg *reg.Registry
}

func NewTemplate[T any](name string) (*Template[T], error) {
	v, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Reg: reg.NewRegistry(name)}, nil
}
