// internal/policy/profiles/policy.go
// Package profiles defines the supervisor-level profile access policy.
// It controls which profile fields may be returned over the IPC bridge
// and enforces consistency checks before a profile becomes active.
package profiles

import "ghost-silicon/pkg/identity"

// Policy controls profile access and field exposure rules.
type Policy struct {
	// AllowNoiseSeeds permits returning noise seeds to the renderer.
	// Set false to disable all noise injection for a session.
	AllowNoiseSeeds bool

	// AllowGPUExposure permits returning GPU vendor/renderer strings.
	AllowGPUExposure bool

	// EnforceConsistency runs cross-field consistency checks before
	// a profile is activated.
	EnforceConsistency bool
}

// DefaultPolicy returns a permissive policy that allows all fields and
// enforces consistency.
func DefaultPolicy() *Policy {
	return &Policy{
		AllowNoiseSeeds:    true,
		AllowGPUExposure:   true,
		EnforceConsistency: true,
	}
}

// Validate checks whether p passes the policy's activation rules.
// Returns a non-nil error when EnforceConsistency is true and the profile
// has inconsistent fields.
func (pol *Policy) Validate(p *identity.Profile) error {
	// Run the standard field-level validator first.
	result := identity.Validate(p)
	if !result.Valid() {
		// Return as a single wrapped error.
		return &ValidationError{result.Error()}
	}

	// Run cross-field consistency checks.
	if pol.EnforceConsistency {
		cr := identity.CheckConsistency(p)
		if !cr.Clean() {
			return &ConsistencyError{cr.Errors[0].Error()}
		}
	}
	return nil
}

// ValidationError wraps a profile validation failure.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return "profile policy: " + e.msg }

// ConsistencyError wraps a profile consistency failure.
type ConsistencyError struct{ msg string }

func (e *ConsistencyError) Error() string { return "profile consistency: " + e.msg }
