// test/identity/rotation_test.go
package identity_test

import (
	"testing"
	"time"

	"ghost-silicon/pkg/identity"
)

func TestRotationPolicy_Validate_Never(t *testing.T) {
	p := identity.RotationPolicy{Trigger: identity.RotationNever}
	if err := p.Validate(); err != nil {
		t.Fatalf("RotationNever should be valid: %v", err)
	}
}

func TestRotationPolicy_Validate_OnSession(t *testing.T) {
	p := identity.RotationPolicy{Trigger: identity.RotationOnSession}
	if err := p.Validate(); err != nil {
		t.Fatalf("RotationOnSession should be valid: %v", err)
	}
}

func TestRotationPolicy_Validate_OnInterval_MissingInterval(t *testing.T) {
	p := identity.RotationPolicy{Trigger: identity.RotationOnInterval}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for OnInterval with zero interval")
	}
}

func TestRotationPolicy_Validate_OnInterval_Valid(t *testing.T) {
	p := identity.RotationPolicy{
		Trigger:  identity.RotationOnInterval,
		Interval: 1 * time.Hour,
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("valid OnInterval policy failed: %v", err)
	}
}

func TestRotationPolicy_Validate_OnRequestCount_MissingCount(t *testing.T) {
	p := identity.RotationPolicy{Trigger: identity.RotationOnRequestCount}
	if err := p.Validate(); err == nil {
		t.Fatal("expected error for OnRequestCount with zero max_requests")
	}
}

func TestRotationState_NotifyNewSession(t *testing.T) {
	base := identity.Windows11DesktopTemplate()
	policy := identity.RotationPolicy{Trigger: identity.RotationOnSession}

	state, err := identity.NewRotationState(base, policy)
	if err != nil {
		t.Fatalf("NewRotationState: %v", err)
	}

	next, rotated := state.NotifyNewSession()
	if !rotated {
		t.Fatal("expected rotation on new session")
	}
	if next.ID == base.ID {
		t.Error("rotated profile should have a new ID")
	}
}

func TestRotationState_NeverRotates(t *testing.T) {
	base := identity.Windows11DesktopTemplate()
	policy := identity.RotationPolicy{Trigger: identity.RotationNever}

	state, err := identity.NewRotationState(base, policy)
	if err != nil {
		t.Fatalf("NewRotationState: %v", err)
	}

	_, rotated := state.NotifyNewSession()
	if rotated {
		t.Error("RotationNever should not rotate on new session")
	}
}

func TestRotationState_RequestCount(t *testing.T) {
	base := identity.Windows11DesktopTemplate()
	policy := identity.RotationPolicy{
		Trigger:     identity.RotationOnRequestCount,
		MaxRequests: 3,
	}

	state, err := identity.NewRotationState(base, policy)
	if err != nil {
		t.Fatalf("NewRotationState: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, rotated := state.IncrementRequests()
		if rotated {
			t.Errorf("should not rotate before reaching max_requests (call %d)", i+1)
		}
	}

	_, rotated := state.IncrementRequests()
	if !rotated {
		t.Error("expected rotation after reaching max_requests=3")
	}
}
