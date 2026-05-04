// pkg/sandbox/windows_process.go
//go:build windows

package sandbox

import (
	"context"
	"fmt"

	"ghost-silicon/internal/platform/windows/jobobject"
	"ghost-silicon/internal/platform/windows/process"
	"ghost-silicon/internal/platform/windows/token"
	"ghost-silicon/pkg/renderer"
)

type WindowsSandbox struct {
	job            *jobobject.JobObject
	enableToken    bool
	integrityLevel string
}

func NewWindowsSandbox(enableJobObject, enableRestrictedToken bool, integrityLevel string) (*WindowsSandbox, error) {
	s := &WindowsSandbox{
		enableToken:    enableRestrictedToken,
		integrityLevel: integrityLevel,
	}
	if enableJobObject {
		jo, err := jobobject.Create("ghost-silicon-sandbox")
		if err != nil {
			return nil, fmt.Errorf("sandbox/windows: create job object: %w", err)
		}
		s.job = jo
	}
	return s, nil
}

func (s *WindowsSandbox) Start(_ context.Context, opts Options) (renderer.Process, error) {
	var tok *token.RestrictedToken
	if s.enableToken {
		level, err := token.ParseIntegrityLevel(s.integrityLevel)
		if err != nil {
			return nil, fmt.Errorf("sandbox/windows: parse integrity level: %w", err)
		}
		tok, err = token.Build(token.Config{
			IntegrityLevel:   level,
			RemovePrivileges: true,
		})
		if err != nil {
			return nil, fmt.Errorf("sandbox/windows: build token: %w", err)
		}
	}

	lopts := process.LaunchOptions{
		Executable: opts.Executable,
		Args:       opts.Args,
		WorkDir:    opts.WorkDir,
		Env:        opts.Env,
		Token:      tok,
		Job:        s.job,
	}

	proc, err := process.Launch(lopts)
	if err != nil {
		if tok != nil {
			tok.Close() //nolint:errcheck
		}
		return nil, fmt.Errorf("sandbox/windows: launch: %w", err)
	}
	if tok != nil {
		tok.Close() //nolint:errcheck
	}
	return &winSandboxProcess{p: proc}, nil
}

func (s *WindowsSandbox) Close() error {
	if s.job != nil {
		return s.job.Close()
	}
	return nil
}

type winSandboxProcess struct{ p *process.Process }

func (w *winSandboxProcess) PID() uint32                              { return w.p.PID() }
func (w *winSandboxProcess) IsRunning() bool                          { return w.p.IsRunning() }
func (w *winSandboxProcess) Terminate() error                         { return w.p.Terminate(1) }
func (w *winSandboxProcess) Wait(ctx context.Context) (uint32, error) { return w.p.Wait(ctx) }
