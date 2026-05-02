// test/integration/lifecycle_test.go
package integration_test

import (
	"context"
	"testing"
	"time"

	"ghost-silicon/internal/app/lifecycle"
	"ghost-silicon/internal/telemetry/logging"
)

func TestLifecycle_StartStop(t *testing.T) {
	log := logging.Nop()
	lc := lifecycle.NewManager(log)

	started := false
	stopped := false

	lc.OnStart("test-subsystem", func(_ context.Context) error {
		started = true
		return nil
	})
	lc.OnStop("test-subsystem", func(_ context.Context) error {
		stopped = true
		return nil
	})

	ctx := context.Background()
	if err := lc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !started {
		t.Error("expected start hook to be called")
	}
	if lc.State() != lifecycle.AppStateRunning {
		t.Errorf("expected Running state, got %s", lc.State())
	}

	if err := lc.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !stopped {
		t.Error("expected stop hook to be called")
	}
	if lc.State() != lifecycle.AppStateStopped {
		t.Errorf("expected Stopped state, got %s", lc.State())
	}
}

func TestLifecycle_StopOrder_LIFO(t *testing.T) {
	log := logging.Nop()
	lc := lifecycle.NewManager(log)

	order := []string{}

	lc.OnStop("first", func(_ context.Context) error {
		order = append(order, "first")
		return nil
	})
	lc.OnStop("second", func(_ context.Context) error {
		order = append(order, "second")
		return nil
	})
	lc.OnStop("third", func(_ context.Context) error {
		order = append(order, "third")
		return nil
	})

	ctx := context.Background()
	_ = lc.Start(ctx)
	_ = lc.Stop(ctx)

	if len(order) != 3 {
		t.Fatalf("expected 3 stop calls, got %d", len(order))
	}
	// LIFO: third, second, first
	if order[0] != "third" || order[1] != "second" || order[2] != "first" {
		t.Errorf("unexpected stop order: %v (expected [third second first])", order)
	}
}

func TestLifecycle_StartError_Aborts(t *testing.T) {
	log := logging.Nop()
	lc := lifecycle.NewManager(log)

	secondCalled := false

	lc.OnStart("failing", func(_ context.Context) error {
		return context.DeadlineExceeded
	})
	lc.OnStart("should-not-run", func(_ context.Context) error {
		secondCalled = true
		return nil
	})

	if err := lc.Start(context.Background()); err == nil {
		t.Fatal("expected Start to fail")
	}
	if secondCalled {
		t.Error("second hook should not be called after first hook fails")
	}
}

func TestLifecycle_Register_FuncHook(t *testing.T) {
	log := logging.Nop()
	lc := lifecycle.NewManager(log)

	startCalled := false
	stopCalled := false

	pair := lifecycle.FuncHook("test",
		func(_ context.Context) error { startCalled = true; return nil },
		func(_ context.Context) error { stopCalled = true; return nil },
	)
	lc.Register(pair)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_ = lc.Start(ctx)
	_ = lc.Stop(ctx)

	if !startCalled {
		t.Error("start hook not called")
	}
	if !stopCalled {
		t.Error("stop hook not called")
	}
}
