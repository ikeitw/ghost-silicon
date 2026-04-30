// internal/engine/health/backoff.go
// Package health — exponential backoff helper used by health probes and
// the restart loop when waiting between retry attempts.
package health

import (
	"math"
	"time"
)

// Backoff tracks exponential back-off state for a retry loop.
type Backoff struct {
	initial  time.Duration
	max      time.Duration
	current  time.Duration
	attempts int
}

// NewBackoff creates a Backoff starting at initial and capped at max.
func NewBackoff(initial, max time.Duration) *Backoff {
	return &Backoff{initial: initial, max: max, current: initial}
}

// Next returns the duration to wait before the next attempt and advances
// the internal counter.
func (b *Backoff) Next() time.Duration {
	d := b.current
	b.attempts++
	// Double with jitter: next = min(current * 2^attempts, max)
	next := time.Duration(float64(b.initial) * math.Pow(2, float64(b.attempts)))
	if next > b.max {
		next = b.max
	}
	b.current = next
	return d
}

// Reset returns the backoff to its initial state.
func (b *Backoff) Reset() {
	b.current = b.initial
	b.attempts = 0
}

// Attempts returns how many Next() calls have been made since the last Reset.
func (b *Backoff) Attempts() int { return b.attempts }
