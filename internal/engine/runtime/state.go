// internal/engine/runtime/state.go
// Package runtime — renderer runtime state machine.
package runtime

import "sync"

// State is the lifecycle state of the rendering engine.
type State string

const (
	StateIdle     State = "idle"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateStopping State = "stopping"
	StateStopped  State = "stopped"
	StateCrashed  State = "crashed"
)

// StateHolder is a thread-safe container for the current engine state.
type StateHolder struct {
	mu    sync.RWMutex
	state State
}

// NewStateHolder returns a StateHolder in the Idle state.
func NewStateHolder() *StateHolder { return &StateHolder{state: StateIdle} }

// Get returns the current state.
func (s *StateHolder) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Set transitions to the new state.
func (s *StateHolder) Set(next State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = next
}

// Is returns true when the current state equals want.
func (s *StateHolder) Is(want State) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == want
}
