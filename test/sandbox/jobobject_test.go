// test/sandbox/jobobject_test.go
// Non-platform tests for job object policy and limits.
// Windows-specific Job Object syscall tests are in test/windows/jobobject_test.go.
package sandbox_test

import (
	"testing"

	"ghost-silicon/internal/policy/sandbox"
)

func TestFromPolicy_ExtractsLimits(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.MemoryLimitMB = 2048
	p.CPURatePercent = 60

	l := sandbox.FromPolicy(p)
	if l.MemoryLimitMB != 2048 {
		t.Errorf("MemoryLimitMB: got %d, want 2048", l.MemoryLimitMB)
	}
	if l.CPURatePercent != 60 {
		t.Errorf("CPURatePercent: got %d, want 60", l.CPURatePercent)
	}
}

func TestFromPolicy_ZeroLimits(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.MemoryLimitMB = 0
	p.CPURatePercent = 0

	l := sandbox.FromPolicy(p)
	if l.MemoryLimitMB != 0 || l.CPURatePercent != 0 {
		t.Error("expected zero limits from default policy with no caps")
	}
}
