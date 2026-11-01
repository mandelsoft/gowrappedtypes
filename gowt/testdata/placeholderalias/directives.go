package testpkg

import (
	"github.com/example/base"
	"github.com/example/registry"
)

// Use short aliases from the imports above instead of full paths.
// gowt: type Concrete wrapping Template[base.Base via base.New] using registry=registry

var _ = base.New
var _ registry.Registry
