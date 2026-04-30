// internal/engine/adapter/registry.go
// Package adapter — adapter registry.
// Maps engine names to Adapter factories so the supervisor can look up the
// right backend from the config string "engine.executable" / adapter name.
package adapter

import (
	"fmt"
	"sync"

	"ghost-silicon/pkg/renderer"
)

// Factory is a function that builds a renderer.Adapter for a given executable path.
type Factory func(executable string) renderer.Adapter

// Registry maps adapter names to their Factory functions.
// It is safe for concurrent reads; writes must happen before the supervisor starts.
type Registry struct {
	mu       sync.RWMutex
	adapters map[string]Factory
}

// Global is the process-wide adapter registry.
var Global = &Registry{adapters: make(map[string]Factory)}

// Register adds a factory under name. Panics if name is already registered.
func (r *Registry) Register(name string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[name]; exists {
		panic(fmt.Sprintf("adapter registry: %q already registered", name))
	}
	r.adapters[name] = f
}

// Get returns the Adapter for name built with executable, or an error if the
// name is not registered.
func (r *Registry) Get(name, executable string) (renderer.Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.adapters[name]
	if !ok {
		return nil, fmt.Errorf("adapter registry: no adapter registered for %q", name)
	}
	return f(executable), nil
}

// Names returns all registered adapter names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	return names
}

func init() {
	// Register the built-in mock adapter so tests and tooling work without
	// a real renderer binary.
	Global.Register("mock", func(_ string) renderer.Adapter {
		return NewMockAdapter()
	})
}
