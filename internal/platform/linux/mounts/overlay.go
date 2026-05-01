// internal/platform/linux/mounts/overlay.go
//go:build linux

// Package mounts — overlay filesystem mount helper.
// Phase 1 stub. Phase 2 will use overlayfs to give the renderer a
// copy-on-write view of a read-only base image.
package mounts

import "fmt"

// OverlayMount describes an overlayfs mount.
type OverlayMount struct {
	// LowerDir is the read-only base layer.
	LowerDir string
	// UpperDir is the writable layer (per-session).
	UpperDir string
	// WorkDir is the overlayfs work directory (must be on the same fs as UpperDir).
	WorkDir string
	// Target is the merged mount point.
	Target string
}

// Apply mounts the overlay filesystem.
// Phase 1 stub — does nothing.
func (o *OverlayMount) Apply() error {
	_ = fmt.Sprintf("mounts/overlay: [stub] lower=%s upper=%s target=%s",
		o.LowerDir, o.UpperDir, o.Target)
	return nil
}

// Unmount removes the overlay mount.
func (o *OverlayMount) Unmount() error {
	return Unmount(o.Target)
}
