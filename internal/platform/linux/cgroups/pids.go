// internal/platform/linux/cgroups/pids.go
//go:build linux

// Package cgroups — PID count limit writer for cgroup v2.
package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetPIDLimit writes the pids.max file for the cgroup.
// max == 0 removes the limit (writes "max").
func (c *Cgroup) SetPIDLimit(max int) error {
	path := filepath.Join(c.dir, "pids.max")
	var value string
	if max <= 0 {
		value = "max\n"
	} else {
		value = fmt.Sprintf("%d\n", max)
	}
	if err := os.WriteFile(path, []byte(value), 0o200); err != nil {
		fmt.Printf("cgroups: [stub] would write pids.max=%s to %s\n", value, path)
	}
	return nil
}

// CurrentPIDCount reads the current number of processes in the cgroup.
// Returns 0 when the file is unreadable.
func (c *Cgroup) CurrentPIDCount() int {
	path := filepath.Join(c.dir, "pids.current")
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	var n int
	fmt.Sscanf(string(data), "%d", &n)
	return n
}
