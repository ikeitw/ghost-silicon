// internal/app/lifecycle/state.go
package lifecycle

import "sync"

type AppState string

const (
	AppStateBooting  AppState = "booting"
	AppStateRunning  AppState = "running"
	AppStateStopping AppState = "stopping"
	AppStateStopped  AppState = "stopped"
)

type StateHolder struct {
	mu    sync.RWMutex
	state AppState
}

func NewStateHolder() *StateHolder { return &StateHolder{state: AppStateBooting} }

func (s *StateHolder) Get() AppState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *StateHolder) Set(next AppState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = next
}

func (s *StateHolder) Is(want AppState) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == want
}
