// internal/engine/health/health.go
// Package health monitors renderer process liveness and readiness.
// It drives the crash-restart loop via the Backoff helper and emits
// structured log events on state changes.
package health

import (
	"context"
	"fmt"
	"time"

	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/renderer"
)

// Status is the current health state of a renderer process.
type Status string

const (
	StatusUnknown   Status = "unknown"
	StatusStarting  Status = "starting"
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDead      Status = "dead"
)

// Monitor watches a renderer.Process and reports its health status.
type Monitor struct {
	process  renderer.Process
	log      *logging.Logger
	status   Status
	backoff  *Backoff
	interval time.Duration
}

// NewMonitor creates a Monitor for proc with the given poll interval.
func NewMonitor(proc renderer.Process, interval time.Duration, log *logging.Logger) *Monitor {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	return &Monitor{
		process:  proc,
		log:      log.WithComponent("health"),
		status:   StatusStarting,
		backoff:  NewBackoff(1*time.Second, 30*time.Second),
		interval: interval,
	}
}

// Status returns the last known health status.
func (m *Monitor) Status() Status { return m.status }

// Run polls the process liveness on interval until ctx is cancelled.
// It sends Status values to the returned channel whenever the status changes.
func (m *Monitor) Run(ctx context.Context) <-chan Status {
	ch := make(chan Status, 4)
	go func() {
		defer close(ch)
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				next := m.probe()
				if next != m.status {
					m.status = next
					m.log.Info("renderer health changed",
						"status", string(next),
						"pid", m.process.PID(),
					)
					select {
					case ch <- next:
					default:
					}
				}
			}
		}
	}()
	return ch
}

// probe performs a single liveness check.
func (m *Monitor) probe() Status {
	if !m.process.IsRunning() {
		return StatusDead
	}
	return StatusHealthy
}

// WaitReady blocks until the process reports healthy or ctx is cancelled.
func (m *Monitor) WaitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("health: timed out waiting for renderer to become ready")
		}
		if m.process.IsRunning() {
			m.status = StatusHealthy
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
