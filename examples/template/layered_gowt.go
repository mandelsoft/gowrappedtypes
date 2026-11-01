//go:build ignore

// Layered[Base] demonstrates gowt.* placeholder imports.  Instead of the
// PACKAGE. convention, each external dependency is declared as a regular
// import with a gowt. prefix.  The directive maps each placeholder to a
// real package via named entries in the "using" clause.
package template

import "gowt.registry"

// Layered embeds a base type and a Registry from a placeholder import.
type Layered[Base any] struct {
	Base
	Registry *registry.Registry
	Label    string
}

// NewLayered creates a Layered value.  NEW is replaced by the "via"
// constructor; the registry qualifier comes from the gowt.registry
// placeholder import.
func NewLayered[Base any](name string) (*Layered[Base], error) {
	b, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Layered[Base]{
		Base:     b,
		Registry: registry.NewRegistry(),
		Label:    name,
	}, nil
}

func (l *Layered[Base]) GetLabel() string {
	return l.Label
}
