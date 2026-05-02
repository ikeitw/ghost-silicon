// test/renderer/health_test.go
package renderer_test

import (
	"context"
	"testing"
	"time"

	"ghost-silicon/internal/engine/health"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/renderer"
)

func TestMonitor_HealthyProcess(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	log := logging.Nop()
	mon := health.NewMonitor(proc, 50*time.Millisecond, log)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	ch := mon.Run(ctx)

	// Drain any status changes — should stay healthy.
	for {
		select {
		case <-ctx.Done():
			return
		case st, ok := <-ch:
			if !ok {
				return
			}
			if st == health.StatusDead {
				t.Errorf("unexpected StatusDead for running process")
			}
		}
	}
}

func TestMonitor_WaitReady_ImmediatelyReady(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	log := logging.Nop()
	mon := health.NewMonitor(proc, 50*time.Millisecond, log)

	ctx := context.Background()
	if err := mon.WaitReady(ctx, 1*time.Second); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
	if mon.Status() != health.StatusHealthy {
		t.Errorf("expected StatusHealthy, got %s", mon.Status())
	}
}

func TestMonitor_WaitReady_Timeout(t *testing.T) {
	// Use a process that reports IsRunning=false.
	proc := renderer.NewMockProcess(0)
	proc.Running = false
	log := logging.Nop()
	mon := health.NewMonitor(proc, 50*time.Millisecond, log)

	ctx := context.Background()
	err := mon.WaitReady(ctx, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for non-running process")
	}
}

func TestBackoff_Progression(t *testing.T) {
	b := health.NewBackoff(100*time.Millisecond, 1*time.Second)

	first := b.Next()
	if first != 100*time.Millisecond {
		t.Errorf("first backoff: expected 100ms, got %v", first)
	}

	second := b.Next()
	if second <= first {
		t.Errorf("second backoff %v should be > first %v", second, first)
	}
}

func TestBackoff_MaxCap(t *testing.T) {
	b := health.NewBackoff(100*time.Millisecond, 300*time.Millisecond)

	for i := 0; i < 10; i++ {
		d := b.Next()
		if d > 300*time.Millisecond {
			t.Errorf("backoff exceeded max at attempt %d: got %v", i+1, d)
		}
	}
}

func TestBackoff_Reset(t *testing.T) {
	b := health.NewBackoff(100*time.Millisecond, 1*time.Second)
	b.Next()
	b.Next()
	b.Reset()

	if b.Attempts() != 0 {
		t.Errorf("expected 0 attempts after Reset, got %d", b.Attempts())
	}
	first := b.Next()
	if first != 100*time.Millisecond {
		t.Errorf("after Reset, expected 100ms, got %v", first)
	}
}
