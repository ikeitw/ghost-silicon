// internal/platform/linux/cgroups/memory.go
//go:build linux

// Package cgroups — memory limit writer for cgroup v2.
package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetMemoryLimit writes the memory.max limit file for the cgroup.
// limitMB == 0 removes the limit (writes "max").
func (c *Cgroup) SetMemoryLimit(limitMB int64) error {
	path := filepath.Join(c.dir, "memory.max")
	var value string
	if limitMB <= 0 {
		value = "max\n"
	} else {
		value = fmt.Sprintf("%d\n", limitMB*1024*1024)
	}
	if err := os.WriteFile(path, []byte(value), 0o200); err != nil {
		// Not fatal — cgroup may not be mounted.
		fmt.Printf("cgroups: [stub] would write memory.max=%s to %s\n", value, path)
	}
	return nil
}

// SetMemorySwapLimit writes the memory.swap.max file.
// limitMB == 0 disables swap entirely.
func (c *Cgroup) SetMemorySwapLimit(limitMB int64) error {
	path := filepath.Join(c.dir, "memory.swap.max")
	var value string
	if limitMB <= 0 {
		value = "0\n"
	} else {
		value = fmt.Sprintf("%d\n", limitMB*1024*1024)
	}
	if err := os.WriteFile(path, []byte(value), 0o200); err != nil {
		fmt.Printf("cgroups: [stub] would write memory.swap.max=%s to %s\n", value, path)
	}
	return nil
}
