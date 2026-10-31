//go:build ignore

// Package template provides a gowt template for a Wrapper type that
// embeds a parametric base and adds a registry and label.
//
// This file is excluded from normal builds (//go:build ignore) but
// included in module downloads, so other packages can instantiate it
// with a cross-package directive.
package template

// Wrapper[Base] embeds a parametric base type and adds a Registry
// (from the "using" package) and an extra label.
//
// This doc comment is stripped from the generated output.
type Wrapper[Base any] struct {
	Base
	Registry *PACKAGE.Registry
	Label    string
}

// NewWrapper is a generic constructor.  NEW is replaced by the "via"
// constructor; PACKAGE. is replaced by the "using" package alias.
// The name "NewWrapper" contains the template type name, so it gets
// renamed: NewWrapper → New<TargetName> (prefix "New" preserved).
func NewWrapper[Base any](name string) (*Wrapper[Base], error) {
	b, err := NEW(name)
	if err != nil {
		return nil, err
	}
	return &Wrapper[Base]{
		Base:     b,
		Registry: PACKAGE.NewRegistry(),
		Label:    name,
	}, nil
}

// GetLabel is a method on the generic Wrapper type.
// The receiver type Wrapper[Base] is substituted to *<TargetName>.
func (w *Wrapper[Base]) GetLabel() string {
	return w.Label
}

// --- begin non-generic functions ---
// RegisterWrapper is a non-generic function.  It demonstrates:
//   - *TemplateName* substitution in a non-generic function name
//   - *Wrapper type reference in a parameter (substituted)
//   - PACKAGE. as a parameter type
func RegisterWrapper(reg *PACKAGE.Registry, w *Wrapper) {
	reg.Register(w.Label, w)
}

// Describe is a non-generic helper whose name does NOT contain the
// template type name — it stays unchanged in the generated output.
func Describe(w *Wrapper) string {
	return "wrapper:" + w.Label // inline comment preserved
}

// --- end non-generic functions ---
