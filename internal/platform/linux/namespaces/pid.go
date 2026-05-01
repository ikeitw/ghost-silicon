// internal/platform/linux/namespaces/pid.go
//go:build linux

// Package namespaces — PID namespace stub.
// Phase 2 will use CLONE_NEWPID so the renderer sees itself as PID 1
// inside its own namespace and cannot observe host PIDs.
package namespaces

// PIDNamespace represents an isolated PID namespace.
type PIDNamespace struct{}

// NewPIDNamespace creates a PIDNamespace descriptor.
func NewPIDNamespace() *PIDNamespace { return &PIDNamespace{} }

// Apply is a Phase 1 stub — does nothing.
func (n *PIDNamespace) Apply() error { return nil }
