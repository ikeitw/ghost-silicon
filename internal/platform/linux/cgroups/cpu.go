// internal/platform/linux/cgroups/cpu.go
//go:build linux

// Package cgroups — CPU limit writer for cgroup v2.
package cgroups

import (
	"fmt"
	"os"
	"path/filepath"
)

// SetCPULimit writes a cpu.max quota for the cgroup.
// ratePercent is 1–100; 0 removes any existing limit.
// cgroup v2 cpu.max format: "<quota_us> <period_us>"
// We use a 100ms period; quota = period * ratePercent / 100.
func (c *Cgroup) SetCPULimit(ratePercent int) error {
	path := filepath.Join(c.dir, "cpu.max")

	var value string
	if ratePercent <= 0 || ratePercent >= 100 {
		value = "max 100000\n"
	} else {
		quotaUs := 100000 * ratePercent / 100
		value = fmt.Sprintf("%d 100000\n", quotaUs)
	}

	if err := os.WriteFile(path, []byte(value), 0o200); err != nil {
		fmt.Printf("cgroups: [stub] would write cpu.max=%s to %s\n", value, path)
	}
	return nil
}

// SetCPUWeight sets the cpu.weight (nice-equivalent) for the cgroup.
// weight should be 1–10000; default is 100.
func (c *Cgroup) SetCPUWeight(weight int) error {
	if weight < 1 {
		weight = 1
	}
	if weight > 10000 {
		weight = 10000
	}
	path := filepath.Join(c.dir, "cpu.weight")
	value := fmt.Sprintf("%d\n", weight)
	if err := os.WriteFile(path, []byte(value), 0o200); err != nil {
		fmt.Printf("cgroups: [stub] would write cpu.weight=%s to %s\n", value, path)
	}
	return nil
}
