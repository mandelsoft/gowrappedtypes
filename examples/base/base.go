// Package base provides a simple Handler type used as a base for
// gowt wrapper examples.
package base

// Handler processes named requests.
type Handler struct {
	name string
}

// New creates a Handler with the given name.
func New(name string) (*Handler, error) {
	return &Handler{name: name}, nil
}

// Create is an alternative constructor for demonstrating the "via" clause.
func Create(name string) (*Handler, error) {
	return &Handler{name: "created:" + name}, nil
}

func (h *Handler) Name() string { return h.name }
