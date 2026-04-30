// internal/policy/sandbox/limits.go
// Package sandbox — resource limit helpers.
package sandbox

import "fmt"

// ResourceLimits holds the numeric limits applied to the renderer Job Object.
type ResourceLimits struct {
	MemoryLimitMB  int64
	CPURatePercent int
}

// FromPolicy extracts resource limits from a Policy.
func FromPolicy(p *Policy) ResourceLimits {
	return ResourceLimits{
		MemoryLimitMB:  p.MemoryLimitMB,
		CPURatePercent: p.CPURatePercent,
	}
}

// Describe returns a human-readable summary of the limits.
func (l ResourceLimits) Describe() string {
	mem := "unlimited"
	if l.MemoryLimitMB > 0 {
		mem = fmt.Sprintf("%d MB", l.MemoryLimitMB)
	}
	cpu := "unlimited"
	if l.CPURatePercent > 0 {
		cpu = fmt.Sprintf("%d%%", l.CPURatePercent)
	}
	return fmt.Sprintf("memory=%s cpu=%s", mem, cpu)
}
