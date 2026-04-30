// pkg/renderer/session.go
// Package renderer — session state tracker.
// A Session binds a running renderer Process to a profile ID, session ID,
// and filesystem layout so the supervisor has one place to look up everything
// about a running renderer instance.
package renderer

import (
	"sync"
	"time"
)

// SessionState is the lifecycle state of a renderer session.
type SessionState string

const (
	StateStarting SessionState = "starting"
	StateReady    SessionState = "ready"
	StateStopping SessionState = "stopping"
	StateStopped  SessionState = "stopped"
	StateCrashed  SessionState = "crashed"
)

// Session holds the runtime state for one active renderer instance.
// All exported methods are safe for concurrent use.
type Session struct {
	mu          sync.RWMutex
	id          string
	profileID   string
	process     Process
	state       SessionState
	startedAt   time.Time
	stoppedAt   time.Time
	crashCount  int
	userDataDir string
}

// NewSession creates a Session in the Starting state.
func NewSession(id, profileID, userDataDir string, proc Process) *Session {
	return &Session{
		id:          id,
		profileID:   profileID,
		process:     proc,
		state:       StateStarting,
		startedAt:   time.Now(),
		userDataDir: userDataDir,
	}
}

// ID returns the session identifier.
func (s *Session) ID() string { return s.id }

// ProfileID returns the identity profile bound to this session.
func (s *Session) ProfileID() string { return s.profileID }

// Process returns the underlying renderer process handle.
func (s *Session) Process() Process { return s.process }

// State returns the current session lifecycle state.
func (s *Session) State() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// SetState transitions the session to a new state.
func (s *Session) SetState(state SessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	if state == StateStopped || state == StateCrashed {
		s.stoppedAt = time.Now()
	}
}

// RecordCrash increments the crash counter and sets state to Crashed.
func (s *Session) RecordCrash() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.crashCount++
	s.state = StateCrashed
	s.stoppedAt = time.Now()
}

// CrashCount returns the number of times this session has crashed.
func (s *Session) CrashCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.crashCount
}

// Uptime returns how long the session has been running.
func (s *Session) Uptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.stoppedAt.IsZero() {
		return s.stoppedAt.Sub(s.startedAt)
	}
	return time.Since(s.startedAt)
}

// UserDataDir returns the isolated filesystem root for this session.
func (s *Session) UserDataDir() string { return s.userDataDir }

// IsActive returns true when the session is Starting or Ready.
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == StateStarting || s.state == StateReady
}
