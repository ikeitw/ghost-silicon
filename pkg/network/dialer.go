// pkg/network/dialer.go
// Package network — custom TCP dialer.
// Wraps net.Dialer with configurable timeouts, DNS override, and connection
// policy. All renderer TCP connections are established through this dialer.
package network

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DialerOptions configures the custom dialer.
type DialerOptions struct {
	// DialTimeout is the maximum time to wait for a TCP connection.
	DialTimeout time.Duration

	// TLSHandshakeTimeout caps the TLS handshake phase.
	TLSHandshakeTimeout time.Duration

	// ResponseHeaderTimeout caps the wait for the first response byte.
	ResponseHeaderTimeout time.Duration

	// KeepAlive is the TCP keep-alive interval.
	KeepAlive time.Duration

	// Resolver is an optional custom DNS resolver. If nil, the system
	// resolver is used.
	Resolver *Resolver
}

// Dialer wraps net.Dialer with ghost-silicon policy.
type Dialer struct {
	inner    *net.Dialer
	opts     DialerOptions
	resolver *Resolver
}

// NewDialer builds a Dialer from opts.
// Missing values receive safe defaults.
func NewDialer(opts DialerOptions) *Dialer {
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 10 * time.Second
	}
	if opts.TLSHandshakeTimeout == 0 {
		opts.TLSHandshakeTimeout = 10 * time.Second
	}
	if opts.ResponseHeaderTimeout == 0 {
		opts.ResponseHeaderTimeout = 30 * time.Second
	}
	if opts.KeepAlive == 0 {
		opts.KeepAlive = 30 * time.Second
	}

	d := &net.Dialer{
		Timeout:   opts.DialTimeout,
		KeepAlive: opts.KeepAlive,
	}

	if opts.Resolver != nil {
		d.Resolver = opts.Resolver.netResolver()
	}

	return &Dialer{inner: d, opts: opts, resolver: opts.Resolver}
}

// DialContext establishes a TCP connection to the given address.
// It satisfies the DialContext field of http.Transport.
func (d *Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Validate network type — we only want TCP.
	switch network {
	case "tcp", "tcp4", "tcp6":
	default:
		return nil, fmt.Errorf("dialer: unsupported network %q", network)
	}

	conn, err := d.inner.DialContext(ctx, network, addr)
	if err != nil {
		return nil, fmt.Errorf("dialer: connect to %s: %w", addr, err)
	}
	return conn, nil
}
