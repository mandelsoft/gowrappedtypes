//go:build ignore

package otherpkg

type Template[T any] struct {
	T
	Extra int
}

func NewTemplate[T any](extra int) (*Template[T], error) {
	v, err := NEW(extra)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extra: extra}, nil
}
