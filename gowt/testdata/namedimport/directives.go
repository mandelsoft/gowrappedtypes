package testpkg

import (
	"github.com/example/base"
	reg "github.com/example/myreg"
)

// The directive uses "reg" — the named import alias — not the full path.
// The template's gowt.registry placeholder should resolve through it.
// gowt: type Concrete wrapping Template[base.Base via base.New] using registry=reg

var _ = base.New
var _ reg.Registry
