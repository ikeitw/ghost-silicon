// internal/app/lifecycle/lifecycle.go
// Package lifecycle manages the application startup and shutdown sequence,
// ensuring subsystems are started and stopped in the correct order.
package lifecycle

import (
	"context"
	"fmt"
	"sync"

	"ghost-silicon/internal/telemetry/logging"
)

// Hook is a named startup or shutdown function.
type Hook struct {
	Name string
	Fn   func(ctx context.Context) error
}

// Manager runs startup hooks in registration order and shutdown hooks in
// reverse order, providing a clean ordered lifecycle for all subsystems.
type Manager struct {
	mu        sync.Mutex
	startups  []Hook
	shutdowns []Hook
	state     *StateHolder
	log       *logging.Logger
}

// NewManager creates a lifecycle Manager.
func NewManager(log *logging.Logger) *Manager {
	return &Manager{
		state: NewStateHolder(),
		log:   log.WithComponent("lifecycle"),
	}
}

// OnStart registers a hook to run during startup.
// Hooks run in registration order.
func (m *Manager) OnStart(name string, fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startups = append(m.startups, Hook{Name: name, Fn: fn})
}

// OnStop registers a hook to run during shutdown.
// Hooks run in reverse registration order (LIFO).
func (m *Manager) OnStop(name string, fn func(ctx context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdowns = append(m.shutdowns, Hook{Name: name, Fn: fn})
}

// Start runs all registered startup hooks in order.
// Stops and returns on the first error.
func (m *Manager) Start(ctx context.Context) error {
	m.state.Set(AppStateBooting)
	m.mu.Lock()
	hooks := make([]Hook, len(m.startups))
	copy(hooks, m.startups)
	m.mu.Unlock()

	for _, h := range hooks {
		m.log.Info("starting subsystem", "name", h.Name)
		if err := h.Fn(ctx); err != nil {
			return fmt.Errorf("lifecycle: start %q: %w", h.Name, err)
		}
		m.log.Info("subsystem ready", "name", h.Name)
	}
	m.state.Set(AppStateRunning)
	return nil
}

// Stop runs all registered shutdown hooks in reverse order.
// All hooks are attempted even if one returns an error.
func (m *Manager) Stop(ctx context.Context) error {
	m.state.Set(AppStateStopping)
	m.mu.Lock()
	hooks := make([]Hook, len(m.shutdowns))
	copy(hooks, m.shutdowns)
	m.mu.Unlock()

	var firstErr error
	for i := len(hooks) - 1; i >= 0; i-- {
		h := hooks[i]
		m.log.Info("stopping subsystem", "name", h.Name)
		if err := h.Fn(ctx); err != nil {
			m.log.Warn("shutdown hook error",
				"name", h.Name,
				logging.FieldError, err.Error(),
			)
			if firstErr == nil {
				firstErr = fmt.Errorf("lifecycle: stop %q: %w", h.Name, err)
			}
		}
	}
	m.state.Set(AppStateStopped)
	return firstErr
}

// State returns the current application lifecycle state.
func (m *Manager) State() AppState { return m.state.Get() }
