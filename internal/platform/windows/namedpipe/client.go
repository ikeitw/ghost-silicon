//go:build windows

package namedpipe

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// pipeAddr implements net.Addr for a named pipe.
type pipeAddr string

func (p pipeAddr) Network() string { return "namedpipe" }
func (p pipeAddr) String() string  { return string(p) }

// pipeConn wraps a named pipe handle as a net.Conn.
type pipeConn struct {
	f    *os.File
	name string
}

func newPipeConn(h windows.Handle, name string) *pipeConn {
	return &pipeConn{
		f:    os.NewFile(uintptr(h), name),
		name: name,
	}
}

func (c *pipeConn) Read(b []byte) (int, error)  { return c.f.Read(b) }
func (c *pipeConn) Write(b []byte) (int, error) { return c.f.Write(b) }
func (c *pipeConn) Close() error                { return c.f.Close() }

func (c *pipeConn) LocalAddr() net.Addr  { return pipeAddr(c.name) }
func (c *pipeConn) RemoteAddr() net.Addr { return pipeAddr(c.name) }

func (c *pipeConn) SetDeadline(t time.Time) error      { return c.f.SetDeadline(t) }
func (c *pipeConn) SetReadDeadline(t time.Time) error  { return c.f.SetReadDeadline(t) }
func (c *pipeConn) SetWriteDeadline(t time.Time) error { return c.f.SetWriteDeadline(t) }

// Client connects to an existing named pipe server.
// Retries for up to timeout if the pipe is busy.
func Client(name string, timeout time.Duration) (net.Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		h, err := openPipe(name)
		if err == nil {
			return newPipeConn(h, name), nil
		}

		if err != windows.ERROR_PIPE_BUSY {
			return nil, fmt.Errorf("namedpipe/client: open %q: %w", name, err)
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("namedpipe/client: timed out waiting for pipe %q", name)
		}

		// WaitNamedPipe blocks until the pipe becomes available or the timeout elapses.
		namePtr, _ := windows.UTF16PtrFromString(name)
		remaining := time.Until(deadline)
		_ = windows.WaitNamedPipe(namePtr, uint32(remaining.Milliseconds()))
	}
}

func openPipe(name string) (windows.Handle, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(
		namePtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED,
		0,
	)
}
