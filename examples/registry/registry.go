// Package registry provides a simple type registry used to demonstrate
// the PACKAGE. placeholder and the "using" clause in gowt templates.
package registry

// Registry tracks named entries.
type Registry struct {
	entries map[string]any
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]any)}
}

func (r *Registry) Register(name string, v any) {
	r.entries[name] = v
}

func (r *Registry) Lookup(name string) (any, bool) {
	v, ok := r.entries[name]
	return v, ok
}
