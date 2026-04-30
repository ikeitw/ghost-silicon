// internal/app/shutdown/drain.go
// Package shutdown — drain helper.
// Drain waits for all in-flight work to finish before the process exits.
package shutdown

import (
	"context"
	"sync"
)

// Drainer tracks in-flight goroutines and waits for them to finish.
type Drainer struct {
	wg sync.WaitGroup
}

// Add increments the in-flight counter by n.
// Must be called before launching the goroutine.
func (d *Drainer) Add(n int) { d.wg.Add(n) }

// Done decrements the in-flight counter by 1.
// Call deferred at the start of every tracked goroutine.
func (d *Drainer) Done() { d.wg.Done() }

// Wait blocks until all in-flight goroutines have called Done,
// or until ctx is cancelled.
func (d *Drainer) Wait(ctx context.Context) {
	finished := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(finished)
	}()
	select {
	case <-finished:
	case <-ctx.Done():
	}
}
