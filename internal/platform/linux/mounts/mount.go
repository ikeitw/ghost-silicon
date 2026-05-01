// internal/platform/linux/mounts/mount.go
//go:build linux

// Package mounts provides filesystem mount helpers for Linux namespace isolation.
// Phase 1: no-op stubs. Phase 2 will call mount(2) syscall directly.
package mounts

import "fmt"

// MountType classifies a mount operation.
type MountType string

const (
	MountTypeBind    MountType = "bind"
	MountTypeOverlay MountType = "overlay"
	MountTypeTmpfs   MountType = "tmpfs"
	MountTypeProc    MountType = "proc"
)

// Mount describes a single mount operation.
type Mount struct {
	Type   MountType
	Source string
	Target string
	Flags  uintptr
	Data   string
}

// Apply performs the mount operation.
// Phase 1 stub — does nothing.
func (m *Mount) Apply() error {
	// TODO Phase 2: syscall.Mount(m.Source, m.Target, string(m.Type), m.Flags, m.Data)
	return nil
}

// Unmount removes a mount at target.
// Phase 1 stub — does nothing.
func Unmount(target string) error {
	// TODO Phase 2: syscall.Unmount(target, 0)
	_ = fmt.Sprintf("mounts: [stub] would unmount %s", target)
	return nil
}
