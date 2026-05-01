// pkg/sandbox/limits.go
// Package sandbox — resource limit helpers shared across all platform backends.
package sandbox

import "fmt"

// Limits holds resource caps that the platform sandbox applies to the renderer.
type Limits struct {
	MemoryLimitMB  int64
	CPURatePercent int
}

// LimitsFromOptions extracts resource limits from sandbox Options.
func LimitsFromOptions(o Options) Limits {
	return Limits{
		MemoryLimitMB:  o.MemoryLimitMB,
		CPURatePercent: o.CPURatePercent,
	}
}

// Validate returns an error when the limits contain out-of-range values.
func (l Limits) Validate() error {
	if l.MemoryLimitMB < 0 {
		return fmt.Errorf("sandbox/limits: memory_limit_mb must be >= 0")
	}
	if l.CPURatePercent < 0 || l.CPURatePercent > 100 {
		return fmt.Errorf("sandbox/limits: cpu_rate_percent must be 0–100, got %d",
			l.CPURatePercent)
	}
	return nil
}

// Describe returns a human-readable summary.
func (l Limits) Describe() string {
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
