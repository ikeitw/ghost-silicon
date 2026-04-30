// internal/engine/launcher/launcher.go
// Package launcher — Windows renderer process launcher.
// Combines argument building, environment construction, Job Object assignment,
// and restricted token application into one Start() call.
package launcher

import (
	"context"
	"fmt"

	"ghost-silicon/internal/config/schema"
	"ghost-silicon/internal/platform/windows/filesystem"
	"ghost-silicon/internal/platform/windows/jobobject"
	"ghost-silicon/internal/platform/windows/process"
	"ghost-silicon/internal/platform/windows/token"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/renderer"
)

// Launcher starts renderer processes with Windows isolation applied.
type Launcher struct {
	cfg *schema.Config
	log *logging.Logger
	job *jobobject.JobObject
}

// New creates a Launcher. If sandbox.enable_job_object is true a shared
// Job Object is created so all renderer processes are killed together on exit.
func New(cfg *schema.Config, log *logging.Logger) (*Launcher, error) {
	l := &Launcher{cfg: cfg, log: log.WithComponent("launcher")}

	if cfg.Sandbox.EnableJobObject {
		jo, err := jobobject.Create(fmt.Sprintf("ghost-silicon-job"))
		if err != nil {
			return nil, fmt.Errorf("launcher: create job object: %w", err)
		}
		limits := &jobobject.Limits{
			MemoryLimitMB:  cfg.Sandbox.MemoryLimitMB,
			CPURatePercent: cfg.Sandbox.CPURatePercent,
		}
		if err := limits.Apply(jo); err != nil {
			jo.Close() //nolint:errcheck
			return nil, fmt.Errorf("launcher: apply job limits: %w", err)
		}
		l.job = jo
		log.Info("job object created")
	}

	return l, nil
}

// Start launches the renderer process for the given session.
func (l *Launcher) Start(_ context.Context, opts renderer.StartOptions) (renderer.Process, error) {
	// Build isolated session directory layout.
	layout, err := filesystem.CreateSessionLayout(l.cfg.Storage.BaseDir, opts.SessionID)
	if err != nil {
		return nil, fmt.Errorf("launcher: create session layout: %w", err)
	}

	// Build command-line arguments.
	args := NewArgBuilder().
		AddSessionArgs(layout.Root, layout.Cache, opts.PipeName).
		AddLogArgs(layout.Logs).
		Extra(l.cfg.Engine.Args).
		Extra(opts.ExtraArgs).
		Build()

	// Build environment block.
	env := NewEnvBuilder().
		SetSession(opts.SessionID, opts.ProfileID, opts.PipeName).
		Build()

	// Build restricted token if configured.
	var tok *token.RestrictedToken
	if l.cfg.Sandbox.EnableRestrictedToken {
		level, err := token.ParseIntegrityLevel(l.cfg.Sandbox.IntegrityLevel)
		if err != nil {
			return nil, fmt.Errorf("launcher: parse integrity level: %w", err)
		}
		tok, err = token.Build(token.Config{
			IntegrityLevel:   level,
			RemovePrivileges: true,
		})
		if err != nil {
			return nil, fmt.Errorf("launcher: build restricted token: %w", err)
		}
	}

	// Launch the process.
	lopts := process.LaunchOptions{
		Executable: l.cfg.Engine.Executable,
		Args:       args,
		WorkDir:    layout.Root,
		Env:        env,
		Token:      tok,
		Job:        l.job,
	}
	proc, err := process.Launch(lopts)
	if err != nil {
		if tok != nil {
			tok.Close() //nolint:errcheck
		}
		return nil, fmt.Errorf("launcher: launch process: %w", err)
	}
	if tok != nil {
		tok.Close() //nolint:errcheck
	}

	l.log.Info("renderer process launched",
		logging.FieldPID, proc.PID(),
		logging.FieldSessionID, opts.SessionID,
	)

	// Wrap the Windows process into the renderer.Process interface.
	return wrapProcess(proc), nil
}

// Close releases the shared Job Object.
func (l *Launcher) Close() error {
	if l.job != nil {
		return l.job.Close()
	}
	return nil
}

// wrapProcess adapts a platform process into renderer.Process.
func wrapProcess(p *process.Process) renderer.Process {
	return &winProcess{p: p}
}

type winProcess struct{ p *process.Process }

func (w *winProcess) PID() uint32                              { return w.p.PID() }
func (w *winProcess) IsRunning() bool                          { return w.p.IsRunning() }
func (w *winProcess) Terminate() error                         { return w.p.Terminate(1) }
func (w *winProcess) Wait(ctx context.Context) (uint32, error) { return w.p.Wait(ctx) }
