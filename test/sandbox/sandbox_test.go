// test/sandbox/sandbox_test.go
package sandbox_test

import (
	"testing"

	"ghost-silicon/pkg/sandbox"
)

func TestLimits_Validate_Valid(t *testing.T) {
	l := sandbox.Limits{MemoryLimitMB: 512, CPURatePercent: 50}
	if err := l.Validate(); err != nil {
		t.Fatalf("expected valid limits, got: %v", err)
	}
}

func TestLimits_Validate_NegativeMemory(t *testing.T) {
	l := sandbox.Limits{MemoryLimitMB: -1}
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for negative MemoryLimitMB")
	}
}

func TestLimits_Validate_CPUOver100(t *testing.T) {
	l := sandbox.Limits{CPURatePercent: 101}
	if err := l.Validate(); err == nil {
		t.Fatal("expected error for CPURatePercent > 100")
	}
}

func TestLimits_Validate_Zero(t *testing.T) {
	l := sandbox.Limits{}
	if err := l.Validate(); err != nil {
		t.Fatalf("zero limits should be valid: %v", err)
	}
}

func TestLimits_Describe_Unlimited(t *testing.T) {
	l := sandbox.Limits{}
	desc := l.Describe()
	if desc != "memory=unlimited cpu=unlimited" {
		t.Errorf("unexpected describe: %q", desc)
	}
}

func TestLimits_Describe_WithValues(t *testing.T) {
	l := sandbox.Limits{MemoryLimitMB: 2048, CPURatePercent: 75}
	desc := l.Describe()
	if desc != "memory=2048 MB cpu=75%" {
		t.Errorf("unexpected describe: %q", desc)
	}
}

func TestLimitsFromOptions(t *testing.T) {
	opts := sandbox.Options{
		MemoryLimitMB:  4096,
		CPURatePercent: 80,
	}
	l := sandbox.LimitsFromOptions(opts)
	if l.MemoryLimitMB != 4096 {
		t.Errorf("MemoryLimitMB: got %d, want 4096", l.MemoryLimitMB)
	}
	if l.CPURatePercent != 80 {
		t.Errorf("CPURatePercent: got %d, want 80", l.CPURatePercent)
	}
}
