//go:build ignore

package testpkg

import "fmt"

// Wrapper has two type parameters: Base (embedded) and Config.
type Wrapper[Base any, Config fmt.Stringer] struct {
	Base
	Cfg Config
}

// NewWrapper creates a Wrapper with the given Config.
func NewWrapper[Base any, Config fmt.Stringer](cfg Config) (*Wrapper[Base, Config], error) {
	v, err := NEW()
	if err != nil {
		return nil, err
	}
	return &Wrapper[Base, Config]{Base: v, Cfg: cfg}, nil
}
