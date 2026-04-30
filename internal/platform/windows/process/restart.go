//go:build windows

package process

import (
	"context"
	"fmt"
	"time"

	"ghost-silicon/internal/telemetry/logging"
)

// RestartPolicy controls how the supervisor reacts to renderer crashes.
type RestartPolicy struct {
	// MaxCrashes is the maximum number of consecutive crashes before giving up.
	MaxCrashes int

	// InitialDelay is the wait time before the first restart attempt.
	InitialDelay time.Duration

	// MaxDelay caps the exponential backoff.
	MaxDelay time.Duration
}

// DefaultRestartPolicy returns conservative restart defaults.
func DefaultRestartPolicy() RestartPolicy {
	return RestartPolicy{
		MaxCrashes:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
	}
}

// Restarter manages the crash-restart loop for a renderer process.
type Restarter struct {
	policy       RestartPolicy
	log          *logging.Logger
	crashCount   int
	currentDelay time.Duration
}

// NewRestarter creates a Restarter with the given policy.
func NewRestarter(policy RestartPolicy, log *logging.Logger) *Restarter {
	return &Restarter{
		policy:       policy,
		log:          log,
		currentDelay: policy.InitialDelay,
	}
}

// RecordCrash records a crash and returns whether the supervisor should
// attempt a restart.  Returns false when the crash limit is exceeded.
func (r *Restarter) RecordCrash(exitCode uint32) bool {
	r.crashCount++
	r.log.Warn("renderer crashed",
		logging.FieldExitCode, exitCode,
		"crash_count", r.crashCount,
		"max_crashes", r.policy.MaxCrashes,
	)
	if r.crashCount > r.policy.MaxCrashes {
		r.log.Error("renderer crash limit exceeded — not restarting",
			"crash_count", r.crashCount,
		)
		return false
	}
	return true
}

// WaitBeforeRestart blocks for the current backoff delay, respecting ctx.
// Returns an error if ctx is cancelled during the wait.
func (r *Restarter) WaitBeforeRestart(ctx context.Context) error {
	r.log.Info("waiting before renderer restart",
		"delay", r.currentDelay.String(),
	)
	select {
	case <-time.After(r.currentDelay):
		r.advanceDelay()
		return nil
	case <-ctx.Done():
		return fmt.Errorf("restart wait cancelled: %w", ctx.Err())
	}
}

// Reset clears the crash counter and delay — call this after a successful
// stable run (e.g. process lived > 30s).
func (r *Restarter) Reset() {
	r.crashCount = 0
	r.currentDelay = r.policy.InitialDelay
}

// CrashCount returns the current consecutive crash count.
func (r *Restarter) CrashCount() int { return r.crashCount }

// advanceDelay doubles the current delay up to MaxDelay.
func (r *Restarter) advanceDelay() {
	r.currentDelay *= 2
	if r.currentDelay > r.policy.MaxDelay {
		r.currentDelay = r.policy.MaxDelay
	}
}
