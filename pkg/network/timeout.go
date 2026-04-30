// pkg/network/timeout.go
// Package network — timeout configuration.
// Centralises all timeout values so every transport uses the same defaults.
package network

import "time"

// TimeoutConfig holds all timeout durations for the network layer.
type TimeoutConfig struct {
	// Dial is the maximum time to establish a TCP connection.
	Dial time.Duration

	// TLSHandshake caps the TLS negotiation phase.
	TLSHandshake time.Duration

	// ResponseHeader caps the wait for the first response byte after the
	// request is fully sent.
	ResponseHeader time.Duration

	// IdleConn is how long an idle keep-alive connection stays in the pool.
	IdleConn time.Duration

	// Request is the end-to-end timeout applied to http.Client calls.
	// 0 means no per-request timeout (rely on context).
	Request time.Duration
}

// DefaultTimeoutConfig returns conservative production-safe defaults.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Dial:           10 * time.Second,
		TLSHandshake:   10 * time.Second,
		ResponseHeader: 30 * time.Second,
		IdleConn:       90 * time.Second,
		Request:        0, // rely on caller context
	}
}
