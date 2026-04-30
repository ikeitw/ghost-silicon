// internal/app/supervisor/worker.go
// Package supervisor — background worker goroutines.
// Workers run for the lifetime of the supervisor and handle tasks like
// profile rotation checks and health monitoring.
package supervisor

import (
	"context"
	"time"

	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/identity"
)

// startRotationWorker starts a background goroutine that polls the rotation
// state on interval and hot-swaps the bridge profile when rotation fires.
func (s *Supervisor) startRotationWorker(ctx context.Context, state *identity.RotationState) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				next, rotated := state.CheckInterval()
				if rotated {
					s.profile = next
					s.bridge.UpdateProfile(next)
					s.log.Info("profile rotated via interval",
						logging.FieldProfileID, next.ID,
						logging.FieldProfileName, next.Name,
					)
				}
			}
		}
	}()
}

// startHealthWorker logs health status changes from the given channel.
func (s *Supervisor) startHealthWorker(ctx context.Context, statusCh <-chan interface{ String() string }) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case st, ok := <-statusCh:
				if !ok {
					return
				}
				s.log.Info("renderer health status", "status", st.String())
			}
		}
	}()
}
