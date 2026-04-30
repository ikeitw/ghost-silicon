// Package jsonrpc implements the JSON-RPC 2.0 server that runs inside the
// supervisor and answers requests from the renderer via the named pipe bridge.
package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"ghost-silicon/internal/ipc/protocol"
	"ghost-silicon/internal/telemetry/logging"
)

// HandlerFunc is a function that handles a single RPC method.
// It receives the raw params JSON and returns a result to serialise, or an error.
type HandlerFunc func(ctx context.Context, params json.RawMessage) (any, error)

// Server routes JSON-RPC requests arriving on a net.Conn to registered
// HandlerFuncs.
type Server struct {
	handlers map[string]HandlerFunc
	log      *logging.Logger
}

// NewServer creates a new JSON-RPC server with the given logger.
func NewServer(log *logging.Logger) *Server {
	return &Server{
		handlers: make(map[string]HandlerFunc),
		log:      log,
	}
}

// Register adds a handler for the given method name.
// Panics if the method has already been registered.
func (s *Server) Register(method string, h HandlerFunc) {
	if _, exists := s.handlers[method]; exists {
		panic(fmt.Sprintf("jsonrpc: method %q already registered", method))
	}
	s.handlers[method] = h
}

// ServeConn reads requests from conn and writes responses until the
// connection closes or ctx is cancelled.
func (s *Server) ServeConn(ctx context.Context, conn net.Conn) {
	defer conn.Close() //nolint:errcheck

	codec := protocol.NewCodec(conn)
	log := s.log.WithComponent("jsonrpc")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, err := codec.ReadRequest()
		if err != nil {
			if err == io.EOF || isClosedConnError(err) {
				log.Debug("client disconnected")
				return
			}
			log.Warn("read request error", logging.FieldError, err.Error())
			return
		}

		log.Debug("rpc request",
			logging.FieldRPCMethod, req.Method,
			logging.FieldRPCID, req.ID,
		)

		s.dispatch(ctx, codec, req)
	}
}

// dispatch calls the handler for req and writes the response.
func (s *Server) dispatch(ctx context.Context, codec *protocol.Codec, req *protocol.Request) {
	handler, ok := s.handlers[req.Method]
	if !ok {
		if !req.IsNotification() {
			_ = codec.WriteError(req.ID, protocol.CodeMethodNotFound,
				fmt.Sprintf("method not found: %s", req.Method))
		}
		return
	}

	result, err := handler(ctx, req.Params)
	if req.IsNotification() {
		return // notifications never get a response
	}

	if err != nil {
		var rpcErr *protocol.RPCError
		if asRPC, ok := err.(*protocol.RPCError); ok {
			rpcErr = asRPC
		} else {
			rpcErr = &protocol.RPCError{
				Code:    protocol.CodeInternalError,
				Message: err.Error(),
			}
		}
		_ = codec.WriteResponse(&protocol.Response{
			ID:    req.ID,
			Error: rpcErr,
		})
		return
	}

	raw, err := json.Marshal(result)
	if err != nil {
		_ = codec.WriteError(req.ID, protocol.CodeInternalError, "failed to marshal result")
		return
	}
	_ = codec.WriteResponse(&protocol.Response{
		ID:     req.ID,
		Result: raw,
	})
}

// Serve accepts connections from ln and serves each in a goroutine.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("jsonrpc: accept: %w", err)
		}
		go s.ServeConn(ctx, conn)
	}
}

func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "use of closed network connection"
}
