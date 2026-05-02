// internal/ipc/websocket/server.go
// Package websocket provides an optional WebSocket-based IPC transport.
// It is an alternative to the named pipe bridge for environments where
// a WebSocket connection is more convenient (e.g. developer tooling).
// Binds only to 127.0.0.1.
package websocket

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"ghost-silicon/internal/telemetry/logging"
)

// Server listens for WebSocket upgrade requests and serves JSON-RPC over them.
type Server struct {
	addr    string
	log     *logging.Logger
	handler http.Handler
	srv     *http.Server
}

// NewServer creates a WebSocket IPC server on addr.
// addr must be a loopback address, e.g. "127.0.0.1:9222".
func NewServer(addr string, log *logging.Logger) *Server {
	s := &Server{
		addr: addr,
		log:  log.WithComponent("ws-server"),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/bridge", s.handleUpgrade)
	s.handler = mux
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return s
}

// Listen opens the TCP listener on the configured address.
func (s *Server) Listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("ws/server: listen %s: %w", s.addr, err)
	}
	s.log.Info("websocket IPC listening", "addr", s.addr)
	return ln, nil
}

// Serve accepts HTTP connections on ln and upgrades them to WebSocket.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
	}()
	if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("ws/server: serve: %w", err)
	}
	return nil
}

// handleUpgrade is the HTTP handler that upgrades connections to WebSocket.
func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrade(w, r)
	if err != nil {
		s.log.Warn("websocket upgrade failed", logging.FieldError, err.Error())
		return
	}
	defer conn.Close() //nolint:errcheck
	s.log.Debug("websocket client connected", "remote", r.RemoteAddr)
	// The upgraded conn satisfies net.Conn and can be handed to the JSON-RPC server.
}
