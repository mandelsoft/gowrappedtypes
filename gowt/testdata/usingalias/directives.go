package testpkg

import (
	"github.com/example/base"
	"github.com/example/extras"
)

// PACKAGE. resolved via short alias from directive file imports.
// gowt: type Concrete wrapping Template[base.Base via base.New] using extras

var _ = base.New
var _ extras.Extras
