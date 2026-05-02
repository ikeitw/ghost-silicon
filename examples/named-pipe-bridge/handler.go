// examples/named-pipe-bridge/handler.go
// Example custom handler that extends the bridge with an application-specific
// method — "app.ping" — which the renderer can call to check liveness.
package main

import (
	"context"
	"encoding/json"
	"time"

	"ghost-silicon/internal/ipc/jsonrpc"
)

// PingResponse is returned by the custom "app.ping" method.
type PingResponse struct {
	Pong      bool   `json:"pong"`
	Timestamp string `json:"timestamp"`
}

// RegisterCustomHandlers adds application-specific methods to srv.
func RegisterCustomHandlers(srv *jsonrpc.Server) {
	srv.Register("app.ping", jsonrpc.NoParams(handlePing))
}

func handlePing(_ context.Context) (any, error) {
	return &PingResponse{
		Pong:      true,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// parseJSON is a helper for examples that need to print JSON results.
func parseJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	out, _ := json.MarshalIndent(v, "  ", "  ")
	return string(out)
}
