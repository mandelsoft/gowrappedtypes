//go:build ignore

package testpkg

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

func TemplateConfig[T any](t *Template[T]) int {
	return t.Extra
}

func NewTemplateConfig[T any](extra int) int {
	return extra
}
