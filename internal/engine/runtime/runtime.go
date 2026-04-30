// internal/engine/runtime/runtime.go
// Package runtime manages the full lifecycle of one renderer engine instance:
// start → health monitoring → crash-restart loop → stop.
package runtime

import (
	"context"
	"fmt"
	"time"

	"ghost-silicon/internal/engine/health"
	"ghost-silicon/internal/platform/windows/process"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/renderer"
)

// Runtime owns one renderer process and drives its lifecycle.
type Runtime struct {
	adapter   renderer.Adapter
	state     *StateHolder
	log       *logging.Logger
	auditor   *audit.Logger
	restarter *process.Restarter
	session   *renderer.Session
}

// Options configures a Runtime.
type Options struct {
	Adapter       renderer.Adapter
	Log           *logging.Logger
	Auditor       *audit.Logger
	RestartPolicy process.RestartPolicy
}

// New creates a Runtime with the given options.
func New(opts Options) *Runtime {
	return &Runtime{
		adapter:   opts.Adapter,
		state:     NewStateHolder(),
		log:       opts.Log.WithComponent("runtime"),
		auditor:   opts.Auditor,
		restarter: process.NewRestarter(opts.RestartPolicy, opts.Log),
	}
}

// Start launches the renderer and enters the supervision loop.
// It blocks until ctx is cancelled or the crash limit is exceeded.
func (r *Runtime) Start(ctx context.Context, opts renderer.StartOptions) error {
	r.state.Set(StateStarting)

	proc, err := r.adapter.Start(ctx, opts)
	if err != nil {
		r.state.Set(StateCrashed)
		return fmt.Errorf("runtime: start renderer: %w", err)
	}

	r.session = renderer.NewSession(opts.SessionID, opts.ProfileID, opts.UserDataDir, proc)
	r.session.SetState(renderer.StateReady)
	r.state.Set(StateRunning)

	r.auditor.Info(audit.EventRendererStarted, "runtime",
		"session_id", opts.SessionID,
		"pid", fmt.Sprintf("%d", proc.PID()),
	)

	return r.supervise(ctx, opts)
}

// supervise waits for the process to exit and restarts it according to policy.
func (r *Runtime) supervise(ctx context.Context, opts renderer.StartOptions) error {
	for {
		exitCode, err := r.session.Process().Wait(ctx)
		if ctx.Err() != nil {
			r.state.Set(StateStopped)
			return nil
		}
		if err != nil {
			r.log.Warn("renderer wait error", logging.FieldError, err.Error())
		}

		r.session.RecordCrash()
		r.auditor.Info(audit.EventRendererCrashed, "runtime",
			"exit_code", fmt.Sprintf("%d", exitCode),
		)

		if !r.restarter.RecordCrash(exitCode) {
			r.state.Set(StateCrashed)
			return renderer.ErrCrashLimitExceeded
		}

		if err := r.restarter.WaitBeforeRestart(ctx); err != nil {
			return nil
		}

		r.log.Info("restarting renderer", "attempt", r.restarter.CrashCount())
		proc, err := r.adapter.Start(ctx, opts)
		if err != nil {
			return fmt.Errorf("runtime: restart renderer: %w", err)
		}
		r.session = renderer.NewSession(opts.SessionID, opts.ProfileID, opts.UserDataDir, proc)
		r.session.SetState(renderer.StateReady)
		r.state.Set(StateRunning)
	}
}

// Stop signals the renderer to stop and waits up to timeout.
func (r *Runtime) Stop(timeout time.Duration) error {
	r.state.Set(StateStopping)
	if r.session == nil {
		r.state.Set(StateStopped)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := r.session.Process().Terminate(); err != nil {
		r.log.Warn("terminate renderer", logging.FieldError, err.Error())
	}
	_, _ = r.session.Process().Wait(ctx)
	r.session.SetState(renderer.StateStopped)
	r.state.Set(StateStopped)
	r.auditor.Info(audit.EventRendererStopped, "runtime", "session_id", r.session.ID())
	return nil
}

// CurrentState returns the runtime's lifecycle state.
func (r *Runtime) CurrentState() State { return r.state.Get() }

// Session returns the active session or nil.
func (r *Runtime) Session() *renderer.Session { return r.session }

// HealthMonitor creates a health.Monitor for the current process.
func (r *Runtime) HealthMonitor(interval time.Duration) *health.Monitor {
	if r.session == nil {
		return nil
	}
	return health.NewMonitor(r.session.Process(), interval, r.log)
}
