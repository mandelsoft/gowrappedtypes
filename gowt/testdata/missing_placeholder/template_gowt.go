//go:build ignore

package testpkg

import "gowt.registry"

type Template[T any] struct {
	T
	Reg *registry.Registry
}
