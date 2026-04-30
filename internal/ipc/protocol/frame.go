// Package protocol defines the wire format used between the ghost-silicon
// supervisor and the renderer over the named pipe IPC bridge.
//
// Wire format (length-prefixed frames):
//
//	┌──────────────────────────┬────────────────────────────────────┐
//	│  4 bytes (uint32 BE)     │  N bytes                           │
//	│  payload length          │  JSON payload                      │
//	└──────────────────────────┴────────────────────────────────────┘
//
// The JSON payload is always a JSON-RPC 2.0 object.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

const (
	// MaxFrameSize is the largest allowed payload (4 MB).
	// Rejects malformed or malicious oversized frames.
	MaxFrameSize = 4 * 1024 * 1024
)

// Frame is a single IPC message.
type Frame struct {
	Payload []byte
}

// ReadFrame reads one length-prefixed frame from r.
func ReadFrame(r io.Reader) (*Frame, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, fmt.Errorf("protocol: read frame length: %w", err)
	}
	length := binary.BigEndian.Uint32(lenBuf[:])
	if length == 0 {
		return nil, fmt.Errorf("protocol: frame length is zero")
	}
	if length > MaxFrameSize {
		return nil, fmt.Errorf("protocol: frame too large (%d > %d)", length, MaxFrameSize)
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("protocol: read frame payload: %w", err)
	}
	return &Frame{Payload: payload}, nil
}

// WriteFrame writes a length-prefixed frame to w.
func WriteFrame(w io.Writer, payload []byte) error {
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("protocol: payload too large (%d > %d)", len(payload), MaxFrameSize)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(payload)))

	if _, err := w.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("protocol: write frame length: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("protocol: write frame payload: %w", err)
	}
	return nil
}

// DecodeJSON unmarshals the frame payload into v.
func (f *Frame) DecodeJSON(v any) error {
	if err := json.Unmarshal(f.Payload, v); err != nil {
		return fmt.Errorf("protocol: decode JSON: %w", err)
	}
	return nil
}

// EncodeJSON creates a frame by marshalling v to JSON.
func EncodeJSON(v any) (*Frame, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("protocol: encode JSON: %w", err)
	}
	return &Frame{Payload: data}, nil
}
