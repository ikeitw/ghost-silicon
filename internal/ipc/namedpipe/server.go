// internal/ipc/namedpipe/server.go
//go:build windows

// Package namedpipe provides the Windows named pipe transport for the
// ghost-silicon IPC bridge. The supervisor runs a pipe server; the
// renderer adapter connects as a client.
package namedpipe

import (
	"context"
	"fmt"
	"net"
	"time"

	"golang.org/x/sys/windows"
)

const (
	// DefaultPipeName is used when no name is configured.
	DefaultPipeName = `\\.\pipe\ghost-silicon-bridge`

	pipeBufferSize = 65536
)

// Server listens on a named pipe and accepts connections from renderer clients.
type Server struct {
	name string
	ln   net.Listener
}

// NewServer creates a Server that will listen on the given pipe name.
func NewServer(name string) *Server {
	if name == "" {
		name = DefaultPipeName
	}
	return &Server{name: name}
}

// Listen opens the named pipe and starts accepting connections.
func (s *Server) Listen() error {
	ln, err := listenPipe(s.name)
	if err != nil {
		return fmt.Errorf("namedpipe/server: listen %q: %w", s.name, err)
	}
	s.ln = ln
	return nil
}

// Accept waits for the next client connection.
func (s *Server) Accept() (net.Conn, error) {
	return s.ln.Accept()
}

// Close shuts down the listener.
func (s *Server) Close() error {
	if s.ln == nil {
		return nil
	}
	return s.ln.Close()
}

// Name returns the pipe name.
func (s *Server) Name() string { return s.name }

// Serve accepts connections in a loop until ctx is cancelled, calling handler
// for each connection in a new goroutine.
func (s *Server) Serve(ctx context.Context, handler func(net.Conn)) error {
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	for {
		conn, err := s.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			return fmt.Errorf("namedpipe/server: accept: %w", err)
		}
		go handler(conn)
	}
}

// listenPipe creates a named pipe listener using CreateNamedPipe.
func listenPipe(name string) (net.Listener, error) {
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, fmt.Errorf("encode pipe name: %w", err)
	}

	h, err := windows.CreateNamedPipe(
		namePtr,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_OVERLAPPED,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		windows.PIPE_UNLIMITED_INSTANCES,
		pipeBufferSize,
		pipeBufferSize,
		0,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateNamedPipe: %w", err)
	}

	return &pipeListener{handle: h, name: name}, nil
}

// pipeListener implements net.Listener over a named pipe instance.
type pipeListener struct {
	handle windows.Handle
	name   string
}

func (l *pipeListener) Accept() (net.Conn, error) {
	err := connectNamedPipe(l.handle)
	if err != nil {
		return nil, fmt.Errorf("ConnectNamedPipe: %w", err)
	}
	return newPipeConn(l.handle, l.name), nil
}

func (l *pipeListener) Close() error {
	return windows.CloseHandle(l.handle)
}

func (l *pipeListener) Addr() net.Addr {
	return pipeAddr(l.name)
}

// connectNamedPipe wraps ConnectNamedPipe synchronously via overlapped I/O.
func connectNamedPipe(h windows.Handle) error {
	var ov windows.Overlapped
	event, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(event) //nolint:errcheck
	ov.HEvent = event

	err = windows.ConnectNamedPipe(h, &ov)
	if err == windows.ERROR_IO_PENDING {
		_, err = windows.WaitForSingleObject(event, uint32(30*time.Second/time.Millisecond))
	}
	if err == windows.ERROR_PIPE_CONNECTED {
		return nil
	}
	return err
}
