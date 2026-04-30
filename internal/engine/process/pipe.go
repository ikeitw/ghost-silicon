// internal/engine/process/pipe.go
// Package process — stdio pipe forwarder.
// Forwards renderer stdout/stderr lines to the supervisor logger so crash
// output is captured in the structured log rather than silently discarded.
package process

import (
	"bufio"
	"context"
	"io"

	"ghost-silicon/internal/telemetry/logging"
)

// ForwardStdio reads lines from r and emits them at DEBUG level on log
// with the given stream label ("stdout" or "stderr").
// Returns when r is closed or ctx is cancelled.
func ForwardStdio(ctx context.Context, r io.Reader, stream string, log *logging.Logger) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			log.Debug("renderer "+stream, "line", scanner.Text())
		}
	}
}
