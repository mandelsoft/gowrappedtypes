//go:build ignore

package testpkg

// Template[T] is a parameterized wrapper that embeds the base type T.
type Template[T any] struct {
	T
	Extra int
}

// NewTemplate creates a Template wrapping a T produced by NEW.
func NewTemplate[T any](extra int) (*Template[T], error) {
	v, err := NEW(extra)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extra: extra}, nil
}
