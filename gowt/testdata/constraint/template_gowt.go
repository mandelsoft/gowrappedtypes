//go:build ignore

package testpkg

import "fmt"

// Template[T] has a non-trivial constraint: T must implement fmt.Stringer.
type Template[T fmt.Stringer] struct {
	T
	Extra int
}

// NewTemplate creates a Template wrapping a T produced by NEW.
func NewTemplate[T fmt.Stringer](extra int) (*Template[T], error) {
	v, err := NEW(extra)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extra: extra}, nil
}
