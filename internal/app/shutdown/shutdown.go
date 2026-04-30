// internal/app/shutdown/shutdown.go
// Package shutdown handles OS signal interception and graceful shutdown
// coordination for the ghost-silicon supervisor process.
package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ghost-silicon/internal/telemetry/logging"
)

// Handler listens for SIGINT / SIGTERM and cancels the root context.
type Handler struct {
	timeout time.Duration
	log     *logging.Logger
}

// NewHandler creates a Handler that allows up to timeout for graceful shutdown.
func NewHandler(timeout time.Duration, log *logging.Logger) *Handler {
	return &Handler{
		timeout: timeout,
		log:     log.WithComponent("shutdown"),
	}
}

// WaitForSignal blocks until SIGINT or SIGTERM is received, then cancels ctx
// and waits for done to be closed or for the timeout to expire.
// Call this from main after all subsystems are started.
func (h *Handler) WaitForSignal(cancel context.CancelFunc, done <-chan struct{}) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	sig := <-sigCh
	h.log.Info("received signal — shutting down", "signal", sig.String())
	cancel()

	select {
	case <-done:
		h.log.Info("graceful shutdown complete")
	case <-time.After(h.timeout):
		h.log.Warn("shutdown timeout exceeded — forcing exit")
		os.Exit(1)
	}
}
