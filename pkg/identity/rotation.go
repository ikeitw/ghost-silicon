package identity

import (
	"fmt"
	"sync"
	"time"
)

// RotationTrigger defines when a profile should be rotated.
type RotationTrigger string

const (
	// RotationNever disables automatic rotation.
	RotationNever RotationTrigger = "never"

	// RotationOnSession rotates the profile at the start of each new session.
	RotationOnSession RotationTrigger = "on_session"

	// RotationOnInterval rotates after a fixed wall-clock interval.
	RotationOnInterval RotationTrigger = "on_interval"

	// RotationOnRequestCount rotates after N requests have been made.
	RotationOnRequestCount RotationTrigger = "on_request_count"
)

// RotationPolicy describes when and how a profile is replaced.
type RotationPolicy struct {
	// Trigger controls what causes a rotation.
	Trigger RotationTrigger `json:"trigger" yaml:"trigger"`

	// Interval is used when Trigger == RotationOnInterval.
	// Accepts values like "1h", "30m", "24h".
	Interval time.Duration `json:"interval,omitempty" yaml:"interval,omitempty"`

	// MaxRequests is used when Trigger == RotationOnRequestCount.
	MaxRequests int64 `json:"max_requests,omitempty" yaml:"max_requests,omitempty"`

	// TemplateSource is the named template used to generate replacement profiles.
	// If empty, the same template as the current profile is used.
	TemplateSource TemplateName `json:"template_source,omitempty" yaml:"template_source,omitempty"`

	// GeneratorSeed seeds the replacement generator (0 = random).
	GeneratorSeed int64 `json:"generator_seed,omitempty" yaml:"generator_seed,omitempty"`
}

// Validate returns an error if the policy is misconfigured.
func (rp *RotationPolicy) Validate() error {
	switch rp.Trigger {
	case RotationNever, RotationOnSession:
		// no extra fields required
	case RotationOnInterval:
		if rp.Interval <= 0 {
			return fmt.Errorf("rotation: trigger %q requires interval > 0", rp.Trigger)
		}
	case RotationOnRequestCount:
		if rp.MaxRequests <= 0 {
			return fmt.Errorf("rotation: trigger %q requires max_requests > 0", rp.Trigger)
		}
	default:
		return fmt.Errorf("rotation: unknown trigger %q", rp.Trigger)
	}
	return nil
}

// RotationState tracks runtime state for a single profile slot.
// It is safe for concurrent use.
type RotationState struct {
	mu           sync.Mutex
	policy       RotationPolicy
	current      *Profile
	activeSince  time.Time
	requestCount int64
	gen          *Generator
}

// NewRotationState initialises state for the given profile and policy.
func NewRotationState(initial *Profile, policy RotationPolicy) (*RotationState, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	seed := policy.GeneratorSeed
	return &RotationState{
		policy:      policy,
		current:     initial,
		activeSince: time.Now(),
		gen:         NewGenerator(seed),
	}, nil
}

// Current returns the active profile.
func (rs *RotationState) Current() *Profile {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return rs.current
}

// IncrementRequests records one outbound network request and rotates the
// profile if the request-count threshold is reached.
// Returns (new profile, true) when a rotation occurred.
func (rs *RotationState) IncrementRequests() (*Profile, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.requestCount++

	if rs.policy.Trigger == RotationOnRequestCount &&
		rs.requestCount >= rs.policy.MaxRequests {
		next := rs.rotate()
		return next, true
	}
	return rs.current, false
}

// CheckInterval checks whether an interval-based rotation is due and
// rotates if so.  Safe to call from a background goroutine on a ticker.
// Returns (new profile, true) when a rotation occurred.
func (rs *RotationState) CheckInterval() (*Profile, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.policy.Trigger != RotationOnInterval {
		return rs.current, false
	}
	if time.Since(rs.activeSince) < rs.policy.Interval {
		return rs.current, false
	}
	next := rs.rotate()
	return next, true
}

// NotifyNewSession rotates if the policy is RotationOnSession.
// Returns (new profile, true) when a rotation occurred.
func (rs *RotationState) NotifyNewSession() (*Profile, bool) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.policy.Trigger != RotationOnSession {
		return rs.current, false
	}
	next := rs.rotate()
	return next, true
}

// rotate generates a replacement profile and resets counters.
// Must be called with rs.mu held.
func (rs *RotationState) rotate() *Profile {
	src := rs.policy.TemplateSource
	if src == "" {
		// Re-use the same template name if stored in tags.
		for _, tag := range rs.current.Tags {
			if tn := TemplateName(tag); FromTemplate(tn) != nil {
				src = tn
				break
			}
		}
	}
	if src == "" {
		src = TemplateWindows11Desktop
	}

	next := rs.gen.Generate(&GeneratorOptions{
		Template: FromTemplate(src),
	})

	rs.current = next
	rs.activeSince = time.Now()
	rs.requestCount = 0
	return next
}
