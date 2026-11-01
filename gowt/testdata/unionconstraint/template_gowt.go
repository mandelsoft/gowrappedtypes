//go:build ignore

package testpkg

// Template[T] has a union type constraint.
type Template[T int | string] struct {
	T
	Extra int
}

// NewTemplate creates a Template wrapping a T produced by NEW.
func NewTemplate[T int | string](extra int) (*Template[T], error) {
	v, err := NEW(extra)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extra: extra}, nil
}
