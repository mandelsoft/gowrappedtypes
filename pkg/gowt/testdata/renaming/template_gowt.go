//go:build ignore

package testpkg

// Template[T] is a parameterized wrapper that embeds the base type T.
type Template[T any] struct {
	T
}

// New creates a Template wrapping a T produced by NEW.
// The "renaming" clause can rename New to any target name.
func New[T any](x int) (*Template[T], error) {
	v, err := NEW(x)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v}, nil
}
