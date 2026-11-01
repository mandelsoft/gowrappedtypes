//go:build ignore

package testpkg

import "fmt"

type Wrapper[Base any, Config fmt.Stringer] struct {
	Base
	Cfg Config
}
