// internal/ipc/websocket/client.go
// Package websocket — WebSocket IPC client.
// Used by developer tools and the renderer mock to connect to the supervisor's
// optional WebSocket bridge endpoint.
package websocket

import (
	"fmt"
	"net"
	"net/http"
	"time"
)

// Client connects to the supervisor's WebSocket IPC endpoint.
type Client struct {
	addr    string
	timeout time.Duration
}

// NewClient creates a Client targeting addr (e.g. "127.0.0.1:9222").
func NewClient(addr string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	return &Client{addr: addr, timeout: timeout}
}

// Connect dials the WebSocket endpoint and returns the upgraded connection.
func (c *Client) Connect() (net.Conn, error) {
	dialer := &net.Dialer{Timeout: c.timeout}
	conn, err := dialer.Dial("tcp", c.addr)
	if err != nil {
		return nil, fmt.Errorf("ws/client: dial %s: %w", c.addr, err)
	}

	// Perform the HTTP upgrade handshake manually so we stay net.Conn compatible.
	req, err := http.NewRequest(http.MethodGet, "http://"+c.addr+"/bridge", nil)
	if err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("ws/client: build request: %w", err)
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "ghost-silicon-ipc")
	req.Header.Set("Sec-WebSocket-Version", "13")

	if err := req.Write(conn); err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("ws/client: write upgrade: %w", err)
	}

	// For the IPC use-case we skip full WebSocket framing and treat the
	// upgraded connection as a raw byte stream (our length-prefixed protocol
	// sits on top). The server side does the same via Upgrade().
	return conn, nil
}
