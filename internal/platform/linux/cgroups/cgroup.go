// internal/platform/linux/cgroups/cgroup.go
//go:build linux

// Package cgroups provides cgroup v2 resource limit helpers for the renderer.
// Phase 1: no-op stubs. Phase 2 will write limit files under
// /sys/fs/cgroup/ghost-silicon/<session-id>/.
package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
)

const cgroupRoot = "/sys/fs/cgroup/ghost-silicon"

// Cgroup represents one cgroup v2 slice for a renderer session.
type Cgroup struct {
	sessionID string
	dir       string
}

// New creates a Cgroup descriptor for sessionID.
func New(sessionID string) *Cgroup {
	return &Cgroup{
		sessionID: sessionID,
		dir:       filepath.Join(cgroupRoot, sessionID),
	}
}

// Create makes the cgroup directory.
// Phase 1 stub — no-op if /sys/fs/cgroup is not writable.
func (c *Cgroup) Create() error {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		// Not fatal on systems where cgroup v2 is unavailable.
		fmt.Printf("cgroups: [stub] would create %s: %v\n", c.dir, err)
	}
	return nil
}

// AddPID moves pid into this cgroup.
// Phase 1 stub — does nothing.
func (c *Cgroup) AddPID(pid uint32) error {
	path := filepath.Join(c.dir, "cgroup.procs")
	data := fmt.Sprintf("%d\n", pid)
	if err := os.WriteFile(path, []byte(data), 0o200); err != nil {
		fmt.Printf("cgroups: [stub] would add PID %d to %s\n", pid, path)
	}
	return nil
}

// Remove deletes the cgroup slice.
func (c *Cgroup) Remove() error {
	if err := os.Remove(c.dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cgroups: remove %s: %w", c.dir, err)
	}
	return nil
}

// Dir returns the cgroup directory path.
func (c *Cgroup) Dir() string { return c.dir }
