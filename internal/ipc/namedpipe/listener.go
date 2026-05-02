// internal/ipc/namedpipe/listener.go
//go:build windows

// Package namedpipe — multi-instance named pipe listener.
// Manages a pool of pipe instances so multiple renderer connections can
// be served concurrently on the same pipe name.
package namedpipe

import (
	"context"
	"fmt"
	"net"

	"ghost-silicon/internal/telemetry/logging"
)

// Listener wraps a named pipe server and accepts connections in a loop.
type Listener struct {
	server *Server
	log    *logging.Logger
}

// NewListener creates a Listener on pipeName.
func NewListener(pipeName string, log *logging.Logger) *Listener {
	return &Listener{
		server: NewServer(pipeName),
		log:    log.WithComponent("namedpipe"),
	}
}

// Listen opens the named pipe for incoming connections.
func (l *Listener) Listen() error {
	if err := l.server.Listen(); err != nil {
		return fmt.Errorf("namedpipe/listener: %w", err)
	}
	l.log.Info("named pipe listening", "pipe", l.server.Name())
	return nil
}

// Serve accepts connections and dispatches each to handler until ctx is cancelled.
func (l *Listener) Serve(ctx context.Context, handler func(net.Conn)) error {
	return l.server.Serve(ctx, func(conn net.Conn) {
		l.log.Debug("new pipe connection")
		handler(conn)
	})
}

// Close shuts down the listener.
func (l *Listener) Close() error {
	return l.server.Close()
}

// PipeName returns the configured pipe name.
func (l *Listener) PipeName() string { return l.server.Name() }
