// internal/ipc/websocket/upgrade.go
// Package websocket — HTTP → WebSocket upgrade helper.
// Performs the WebSocket handshake and returns the raw net.Conn so that
// the length-prefixed JSON-RPC protocol layer can sit on top without
// needing a full WebSocket framing library.
package websocket

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// Upgrade performs the WebSocket upgrade handshake on w/r and returns the
// hijacked net.Conn. The caller owns the connection after this returns.
func Upgrade(w http.ResponseWriter, r *http.Request) (net.Conn, error) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return nil, fmt.Errorf("ws/upgrade: not a websocket request")
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, fmt.Errorf("ws/upgrade: missing Sec-WebSocket-Key")
	}

	accept := computeAccept(key)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, fmt.Errorf("ws/upgrade: ResponseWriter does not support hijacking")
	}

	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("ws/upgrade: hijack: %w", err)
	}

	// Write the 101 Switching Protocols response.
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"

	if _, err := buf.WriteString(response); err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("ws/upgrade: write response: %w", err)
	}
	if err := buf.Flush(); err != nil {
		conn.Close() //nolint:errcheck
		return nil, fmt.Errorf("ws/upgrade: flush response: %w", err)
	}

	return &wsConn{Conn: conn, buf: buf}, nil
}

// wsConn wraps a hijacked connection with its bufio.ReadWriter so buffered
// data is not lost after the upgrade.
type wsConn struct {
	net.Conn
	buf *bufio.ReadWriter
}

func (c *wsConn) Read(b []byte) (int, error)  { return c.buf.Read(b) }
func (c *wsConn) Write(b []byte) (int, error) { return c.Conn.Write(b) }

// computeAccept calculates the Sec-WebSocket-Accept header value.
func computeAccept(key string) string {
	h := sha1.New() //nolint:gosec
	h.Write([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
