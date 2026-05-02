// internal/platform/darwin/sandbox/sandbox.go
//go:build darwin

// Package sandbox provides the macOS sandbox profile stub for the renderer.
// Phase 1: no-op. Phase 2 will use sandbox-exec with a custom profile.
package sandbox

import "fmt"

// Sandbox manages the macOS sandbox profile for one renderer session.
type Sandbox struct {
	sessionID string
	profile   *Profile
}

// New creates a Sandbox for sessionID using the given profile.
func New(sessionID string, p *Profile) *Sandbox {
	return &Sandbox{sessionID: sessionID, profile: p}
}

// Apply activates the sandbox profile.
// Phase 1 stub — does nothing.
func (s *Sandbox) Apply() error {
	fmt.Printf("darwin/sandbox: [stub] would apply profile %q for session %s\n",
		s.profile.Name, s.sessionID)
	return nil
}

// Remove tears down the sandbox profile.
// Phase 1 stub — does nothing.
func (s *Sandbox) Remove() error {
	return nil
}

// WrapCommand prepends sandbox-exec args to the given command.
// Phase 1 stub — returns the command unchanged.
func (s *Sandbox) WrapCommand(executable string, args []string) (string, []string) {
	// Phase 2: return "sandbox-exec", append("-f", profilePath, executable, args...)
	return executable, args
}
