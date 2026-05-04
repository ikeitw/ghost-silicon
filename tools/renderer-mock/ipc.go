// tools/renderer-mock/ipc.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"ghost-silicon/internal/ipc/protocol"
)

type BridgeClient struct {
	conn      net.Conn
	codec     *protocol.Codec
	nextID    atomic.Int64
	sessionID string
}

func NewBridgeClient(pipeName string, timeout time.Duration) (*BridgeClient, error) {
	conn, err := dialPipe(pipeName, timeout)
	if err != nil {
		return nil, fmt.Errorf("ipc: dial %q: %w", pipeName, err)
	}
	return &BridgeClient{
		conn:      conn,
		codec:     protocol.NewCodec(conn),
		sessionID: "mock-renderer-session",
	}, nil
}

func (c *BridgeClient) Call(ctx context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	req := &protocol.Request{
		JSONRPC: protocol.JSONRPCVersion,
		Method:  method,
		Params:  params,
		ID:      protocol.NewIntID(id),
	}
	if err := c.codec.WriteRequest(req); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = c.conn.SetReadDeadline(dl)
	} else {
		_ = c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	}
	resp, err := c.codec.ReadResponse()
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp.Result, nil
}

func (c *BridgeClient) SessionID() string { return c.sessionID }
func (c *BridgeClient) Close() error      { return c.conn.Close() }
