//go:build ignore

package testpkg

type Template[T any] struct {
	T
	Extras PACKAGE.Extras
}

func NewTemplate[T any](name string) (*Template[T], error) {
	v, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Template[T]{T: v, Extras: *PACKAGE.NewExtras(name)}, nil
}
