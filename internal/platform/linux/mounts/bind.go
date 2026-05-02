// internal/platform/linux/mounts/bind.go
//go:build linux

// Package mounts — bind mount helper.
// Phase 1 stub. Phase 2 will use MS_BIND to bind-mount session directories
// into the renderer's private mount namespace.
package mounts

import "fmt"

// BindMount describes a bind mount operation.
type BindMount struct {
	// Source is the host directory to expose.
	Source string
	// Target is the mount point inside the renderer's namespace.
	Target string
	// ReadOnly makes the bind mount read-only.
	ReadOnly bool
}

// Apply performs the bind mount.
// Phase 1 stub — does nothing.
func (b *BindMount) Apply() error {
	_ = fmt.Sprintf("mounts/bind: [stub] %s → %s (ro=%v)", b.Source, b.Target, b.ReadOnly)
	return nil
}

// Unmount removes the bind mount.
func (b *BindMount) Unmount() error {
	return Unmount(b.Target)
}
