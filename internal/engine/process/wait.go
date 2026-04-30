// internal/engine/process/wait.go
// Package process — cross-platform wait helper used by the engine layer.
// The actual OS wait is delegated to the platform handle; this file provides
// a timeout-aware wrapper that the runtime loop calls.
package process

import (
	"context"
	"fmt"
	"time"
)

// WaitWithTimeout calls wait with a deadline derived from timeout.
// If timeout <= 0 the call blocks until the process exits or ctx is cancelled.
func WaitWithTimeout(
	ctx context.Context,
	wait func(context.Context) (uint32, error),
	timeout time.Duration,
) (uint32, error) {
	if timeout <= 0 {
		return wait(ctx)
	}
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	code, err := wait(tCtx)
	if err != nil && tCtx.Err() != nil {
		return 0, fmt.Errorf("process: wait timed out after %s", timeout)
	}
	return code, err
}
