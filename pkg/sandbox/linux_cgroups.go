// pkg/sandbox/linux_cgroups.go
//go:build linux

// Package sandbox — Linux cgroup resource limit stub.
// Phase 1: no-op. Phase 2 will write proper cgroup v2 limit files.
package sandbox

import "fmt"

// ApplyCgroupLimits applies resource limits via cgroups for the given pid.
// Currently a no-op stub for Phase 1.
func ApplyCgroupLimits(pid uint32, l Limits) error {
	if err := l.Validate(); err != nil {
		return fmt.Errorf("sandbox/linux_cgroups: %w", err)
	}
	// TODO Phase 2: write to /sys/fs/cgroup/ghost-silicon/<session>/
	return nil
}
