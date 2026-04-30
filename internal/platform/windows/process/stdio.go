//go:build windows

package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// StdioPipes holds the read ends of the renderer's stdout and stderr pipes.
// The write ends are passed to the child process via StartupInfo.
type StdioPipes struct {
	StdoutRead *os.File
	StderrRead *os.File

	// These are closed after the child process is created.
	stdoutWrite windows.Handle
	stderrWrite windows.Handle
}

// CreateStdioPipes creates two anonymous pipes for stdout and stderr capture.
// The write ends are inheritable so the child process can write to them.
func CreateStdioPipes() (*StdioPipes, error) {
	var (
		stdoutR, stdoutW windows.Handle
		stderrR, stderrW windows.Handle
	)

	// Make the write ends inheritable by the child process.
	sa := &windows.SecurityAttributes{InheritHandle: 1}
	sa.Length = uint32(12) // sizeof(SECURITY_ATTRIBUTES)

	if err := windows.CreatePipe(&stdoutR, &stdoutW, sa, 0); err != nil {
		return nil, fmt.Errorf("stdio: create stdout pipe: %w", err)
	}
	if err := windows.CreatePipe(&stderrR, &stderrW, sa, 0); err != nil {
		_ = windows.CloseHandle(stdoutR)
		_ = windows.CloseHandle(stdoutW)
		return nil, fmt.Errorf("stdio: create stderr pipe: %w", err)
	}

	// Ensure our read ends are NOT inherited by the child.
	_ = windows.SetHandleInformation(stdoutR, windows.HANDLE_FLAG_INHERIT, 0)
	_ = windows.SetHandleInformation(stderrR, windows.HANDLE_FLAG_INHERIT, 0)

	return &StdioPipes{
		StdoutRead:  os.NewFile(uintptr(stdoutR), "stdout-r"),
		StderrRead:  os.NewFile(uintptr(stderrR), "stderr-r"),
		stdoutWrite: stdoutW,
		stderrWrite: stderrW,
	}, nil
}

// ApplyToStartupInfo sets the pipe write ends in si so the child writes
// its output into our pipes.
func (p *StdioPipes) ApplyToStartupInfo(si *windows.StartupInfo) {
	si.StdOutput = p.stdoutWrite
	si.StdErr = p.stderrWrite
	si.Flags |= windows.STARTF_USESTDHANDLES
}

// CloseWriteEnds closes the write ends of both pipes.  This must be called
// after the child process is created so that EOF is properly delivered when
// the child exits.
func (p *StdioPipes) CloseWriteEnds() {
	_ = windows.CloseHandle(p.stdoutWrite)
	_ = windows.CloseHandle(p.stderrWrite)
	p.stdoutWrite = windows.InvalidHandle
	p.stderrWrite = windows.InvalidHandle
}

// ForwardTo reads lines from both pipes and forwards them to dst until ctx
// is cancelled or both pipes are closed.  Each line is prefixed with
// "stdout: " or "stderr: ".
func (p *StdioPipes) ForwardTo(ctx context.Context, dst io.Writer) {
	pipe := func(prefix string, r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
				_, _ = fmt.Fprintf(dst, "%s: %s\n", prefix, scanner.Text())
			}
		}
	}

	done := make(chan struct{}, 2)
	go func() { pipe("stdout", p.StdoutRead); done <- struct{}{} }()
	go func() { pipe("stderr", p.StderrRead); done <- struct{}{} }()

	for i := 0; i < 2; i++ {
		select {
		case <-done:
		case <-ctx.Done():
			return
		}
	}
}
