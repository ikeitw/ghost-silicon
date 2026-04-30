package jsonrpc

import (
	"context"
	"encoding/json"
	"fmt"

	"ghost-silicon/internal/ipc/protocol"
)

// DecodeParams unmarshals the raw params JSON into v.
// Returns a properly coded RPCError on failure.
func DecodeParams(raw json.RawMessage, v any) error {
	if err := json.Unmarshal(raw, v); err != nil {
		return &protocol.RPCError{
			Code:    protocol.CodeInvalidParams,
			Message: fmt.Sprintf("invalid params: %v", err),
		}
	}
	return nil
}

// NoParams is the handler for methods that take no parameters.
// fn is called with only the context.
func NoParams(fn func(ctx context.Context) (any, error)) HandlerFunc {
	return func(ctx context.Context, _ json.RawMessage) (any, error) {
		return fn(ctx)
	}
}

// WithParams decodes params into a value of type T before calling fn.
func WithParams[T any](fn func(ctx context.Context, params T) (any, error)) HandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (any, error) {
		var p T
		if err := DecodeParams(raw, &p); err != nil {
			return nil, err
		}
		return fn(ctx, p)
	}
}
