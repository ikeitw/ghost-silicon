// pkg/api/server.go
// Package api provides the optional local HTTP control API for ghost-silicon.
// It binds only to 127.0.0.1 so no external access is possible.
// The API lets a GUI, CLI tool, or dashboard query and control the supervisor.
package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"ghost-silicon/internal/telemetry/logging"
)

// Server is the local HTTP control API server.
type Server struct {
	addr    string
	srv     *http.Server
	log     *logging.Logger
	handler http.Handler
}

// NewServer creates a Server listening on addr.
// addr must be a loopback address, e.g. "127.0.0.1:9223".
func NewServer(addr string, readTimeout, writeTimeout time.Duration, log *logging.Logger) (*Server, error) {
	mux := http.NewServeMux()
	s := &Server{
		addr:    addr,
		log:     log.WithComponent("api"),
		handler: mux,
	}
	s.srv = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}
	return s, nil
}

// RegisterRoutes mounts all API route handlers onto the server mux.
// Must be called before Listen.
func (s *Server) RegisterRoutes(deps *Deps) {
	mux := s.srv.Handler.(*http.ServeMux)
	registerRoutes(mux, deps)
}

// Listen starts the HTTP listener and returns.
// Call Serve to begin accepting connections.
func (s *Server) Listen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("api: listen %s: %w", s.addr, err)
	}
	s.log.Info("local API listening", "addr", s.addr)
	return ln, nil
}

// Serve accepts connections on ln until ctx is cancelled.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
	}()
	if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api: serve: %w", err)
	}
	return nil
}
