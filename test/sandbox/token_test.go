// test/sandbox/token_test.go
// This file contains non-platform-specific token policy tests.
// Windows-specific token creation tests are in test/windows/token_test.go.
package sandbox_test

import (
	"testing"

	"ghost-silicon/internal/policy/sandbox"
)

func TestSandboxPolicy_DefaultIsValid(t *testing.T) {
	p := sandbox.DefaultPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default sandbox policy is invalid: %v", err)
	}
}

func TestSandboxPolicy_InvalidIntegrityLevel(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.IntegrityLevel = "super-high"
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for invalid integrity level")
	}
}

func TestSandboxPolicy_NegativeMemory(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.MemoryLimitMB = -1
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for negative memory limit")
	}
}

func TestSandboxPolicy_CPURateOutOfRange(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.CPURatePercent = 150
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for cpu_rate_percent > 100")
	}
}

func TestSandboxPolicy_LowIntegrity_Valid(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.IntegrityLevel = "low"
	if err := p.Validate(); err != nil {
		t.Fatalf("low integrity should be valid: %v", err)
	}
}

func TestResourceLimits_Describe(t *testing.T) {
	p := sandbox.DefaultPolicy()
	p.MemoryLimitMB = 1024
	p.CPURatePercent = 50

	l := sandbox.FromPolicy(p)
	desc := l.Describe()
	if desc == "" {
		t.Error("Describe returned empty string")
	}
}
