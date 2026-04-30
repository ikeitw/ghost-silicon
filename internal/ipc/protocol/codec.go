package protocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// Codec reads and writes JSON-RPC 2.0 messages as length-prefixed frames.
// One Codec per connection; not safe for concurrent use without external
// locking.
type Codec struct {
	rw io.ReadWriter
}

// NewCodec creates a Codec over the given connection.
func NewCodec(rw io.ReadWriter) *Codec {
	return &Codec{rw: rw}
}

// ReadRequest reads the next request from the wire.
func (c *Codec) ReadRequest() (*Request, error) {
	frame, err := ReadFrame(c.rw)
	if err != nil {
		return nil, err
	}
	var req Request
	if err := frame.DecodeJSON(&req); err != nil {
		return nil, fmt.Errorf("codec: decode request: %w", err)
	}
	if req.JSONRPC != JSONRPCVersion {
		return nil, fmt.Errorf("codec: expected jsonrpc %q, got %q",
			JSONRPCVersion, req.JSONRPC)
	}
	return &req, nil
}

// WriteResponse encodes resp and sends it as a framed payload.
func (c *Codec) WriteResponse(resp *Response) error {
	resp.JSONRPC = JSONRPCVersion
	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("codec: marshal response: %w", err)
	}
	return WriteFrame(c.rw, data)
}

// WriteRequest encodes req and sends it as a framed payload.
// Used by the renderer-side client.
func (c *Codec) WriteRequest(req *Request) error {
	req.JSONRPC = JSONRPCVersion
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("codec: marshal request: %w", err)
	}
	return WriteFrame(c.rw, data)
}

// WriteError sends a JSON-RPC error response for the given request ID.
func (c *Codec) WriteError(id *RequestID, code int, msg string) error {
	return c.WriteResponse(&Response{
		ID:    id,
		Error: &RPCError{Code: code, Message: msg},
	})
}
