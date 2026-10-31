package basic

// Registry is a local type matching the PACKAGE.Registry placeholder
// when no "using" clause is present (PACKAGE. is removed, leaving
// the bare identifier).
type Registry struct {
	entries map[string]any
}

func NewRegistry() *Registry {
	return &Registry{entries: make(map[string]any)}
}

func (r *Registry) Register(name string, v any) {
	r.entries[name] = v
}
