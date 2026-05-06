//go:build windows

// Package process provides the Windows-specific renderer process launcher.
// It uses CreateProcessAsUser with the restricted token produced by the token
// package, and assigns the new process to a Job Object immediately on creation.
package process

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"ghost-silicon/internal/platform/windows/jobobject"
	"ghost-silicon/internal/platform/windows/token"
)

// LaunchOptions configures the renderer process launch.
type LaunchOptions struct {
	// Executable is the full path to the renderer binary.
	Executable string

	// Args are the command-line arguments (not including the executable).
	Args []string

	// WorkDir is the working directory.  If empty, the current directory is used.
	WorkDir string

	// Env is the environment block for the new process.
	// If nil, os.Environ() is used.
	Env []string

	// Token is the restricted token to use.  If nil, the current process token
	// is used (no restriction applied).
	Token *token.RestrictedToken

	// Job is the Job Object to assign the process to.  May be nil.
	Job *jobobject.JobObject
}

// Process wraps a launched Windows process.
type Process struct {
	handle  windows.Handle
	pid     uint32
	threadH windows.Handle
}

// PID returns the process identifier.
func (p *Process) PID() uint32 { return p.pid }

// Handle returns the process handle.  Do not close it directly; use Close().
func (p *Process) Handle() windows.Handle { return p.handle }

// Close releases process and thread handles.
func (p *Process) Close() error {
	var errs []string
	if p.threadH != windows.InvalidHandle {
		if err := windows.CloseHandle(p.threadH); err != nil {
			errs = append(errs, "thread handle: "+err.Error())
		}
		p.threadH = windows.InvalidHandle
	}
	if p.handle != windows.InvalidHandle {
		if err := windows.CloseHandle(p.handle); err != nil {
			errs = append(errs, "process handle: "+err.Error())
		}
		p.handle = windows.InvalidHandle
	}
	if len(errs) > 0 {
		return fmt.Errorf("process/close: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Launch starts a new process with the given options.
func Launch(opts LaunchOptions) (*Process, error) {
	if opts.Executable == "" {
		return nil, fmt.Errorf("process/launch: executable must not be empty")
	}

	cmdLine := buildCommandLine(opts.Executable, opts.Args)
	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		return nil, fmt.Errorf("process/launch: encode cmdline: %w", err)
	}

	workDirPtr, err := utf16PtrOrNil(opts.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("process/launch: encode workdir: %w", err)
	}

	envBlock, err := buildEnvBlock(opts.Env)
	if err != nil {
		return nil, fmt.Errorf("process/launch: build env: %w", err)
	}

	si := &windows.StartupInfo{
		Flags: windows.STARTF_USESTDHANDLES,
	}
	si.Cb = uint32(unsafe.Sizeof(*si))

	var pi windows.ProcessInformation

	// CREATE_SUSPENDED lets us assign the Job Object before the main thread runs.
	// CREATE_UNICODE_ENVIRONMENT tells CreateProcess that envBlock is UTF-16.
	const flags = windows.CREATE_SUSPENDED |
		windows.CREATE_UNICODE_ENVIRONMENT |
		windows.CREATE_NEW_PROCESS_GROUP

	if opts.Token != nil {
		err = windows.CreateProcessAsUser(
			opts.Token.Handle(),
			nil,
			cmdLinePtr,
			nil, nil, false,
			flags,
			envBlock,
			workDirPtr,
			si, &pi,
		)
	} else {
		err = windows.CreateProcess(
			nil,
			cmdLinePtr,
			nil, nil, false,
			flags,
			envBlock,
			workDirPtr,
			si, &pi,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("process/launch: CreateProcess: %w", err)
	}

	proc := &Process{
		handle:  pi.Process,
		pid:     pi.ProcessId,
		threadH: pi.Thread,
	}

	// Assign to Job Object before resuming so the process cannot escape.
	if opts.Job != nil {
		if err := opts.Job.AssignProcess(pi.Process); err != nil {
			// Terminate and clean up if job assignment fails.
			_ = windows.TerminateProcess(pi.Process, 1)
			_ = proc.Close()
			return nil, fmt.Errorf("process/launch: assign job object: %w", err)
		}
	}

	// Resume the main thread; the process starts executing now.
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		_ = windows.TerminateProcess(pi.Process, 1)
		_ = proc.Close()
		return nil, fmt.Errorf("process/launch: ResumeThread: %w", err)
	}

	return proc, nil
}

// buildCommandLine constructs a properly quoted command line string.
func buildCommandLine(exe string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, syscall.EscapeArg(exe))
	for _, a := range args {
		parts = append(parts, syscall.EscapeArg(a))
	}
	return strings.Join(parts, " ")
}

// buildEnvBlock converts a []string env slice to a UTF-16 double-null
// terminated environment block suitable for CreateProcess.
// windows.UTF16PtrFromString rejects strings containing embedded null bytes,
// so we build the []uint16 block manually.
func buildEnvBlock(env []string) (*uint16, error) {
	if env == nil {
		env = os.Environ()
	}

	// Each entry is "KEY=VALUE" encoded as UTF-16, terminated by a null uint16.
	// The block ends with an extra null uint16.
	var block []uint16
	for _, s := range env {
		encoded, err := windows.UTF16FromString(s)
		if err != nil {
			return nil, fmt.Errorf("buildEnvBlock: encode %q: %w", s, err)
		}
		// UTF16FromString appends a null terminator already.
		block = append(block, encoded...)
	}
	// Final double-null terminator (UTF16FromString already added one null per
	// entry, so we just need one more to close the block).
	block = append(block, 0)

	return &block[0], nil
}

func utf16PtrOrNil(s string) (*uint16, error) {
	if s == "" {
		return nil, nil
	}
	return windows.UTF16PtrFromString(s)
}
