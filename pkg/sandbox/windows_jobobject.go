// pkg/sandbox/windows_jobobject.go
//go:build windows

// Package sandbox — Windows Job Object helpers at the pkg/sandbox layer.
// Thin wrappers that apply Limits from sandbox.Options to a jobobject.JobObject.
package sandbox

import (
	"fmt"

	"ghost-silicon/internal/platform/windows/jobobject"
)

// ApplyLimitsToJob applies sandbox resource limits to jo.
func ApplyLimitsToJob(jo *jobobject.JobObject, l Limits) error {
	if err := l.Validate(); err != nil {
		return fmt.Errorf("sandbox/jobobject: %w", err)
	}
	limits := &jobobject.Limits{
		MemoryLimitMB:  l.MemoryLimitMB,
		CPURatePercent: l.CPURatePercent,
	}
	return limits.Apply(jo)
}
