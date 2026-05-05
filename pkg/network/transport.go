// pkg/network/transport.go
// Package network provides the custom HTTP transport layer for ghost-silicon.
// All renderer HTTP traffic flows through this transport so that network
// policy, proxy, DNS, header, and timeout rules are consistently enforced.
package network

import (
	"fmt"
	"net/http"
)

// Transport is a policy-aware http.RoundTripper.
// Build one per session using NewTransport; it is safe for concurrent use.
type Transport struct {
	inner   http.RoundTripper
	policy  *Policy
	headers *HeaderPolicy
}

// TransportOptions configures the transport.
type TransportOptions struct {
	Policy    *Policy
	Headers   *HeaderPolicy
	Dialer    *Dialer
	TLSPolicy *TLSPolicy
	Pool      *ConnectionPool
	ProxyFunc ProxyFunc
}

// NewTransport constructs a Transport from the given options.
// Any nil option receives a safe default.
func NewTransport(opts TransportOptions) (*Transport, error) {
	if opts.Policy == nil {
		opts.Policy = DefaultPolicy()
	}
	if opts.Headers == nil {
		opts.Headers = DefaultHeaderPolicy()
	}

	dialer := opts.Dialer
	if dialer == nil {
		dialer = NewDialer(DialerOptions{})
	}

	tlsPolicy := opts.TLSPolicy
	if tlsPolicy == nil {
		tlsPolicy = DefaultTLSPolicy()
	}

	tlsCfg, err := tlsPolicy.TLSConfig()
	if err != nil {
		return nil, fmt.Errorf("network/transport: build TLS config: %w", err)
	}

	pool := opts.Pool
	if pool == nil {
		pool = DefaultConnectionPool()
	}

	inner := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSClientConfig:       tlsCfg,
		MaxIdleConns:          pool.MaxIdleConns,
		MaxIdleConnsPerHost:   pool.MaxIdleConnsPerHost,
		MaxConnsPerHost:       pool.MaxConnsPerHost,
		IdleConnTimeout:       pool.IdleConnTimeout,
		TLSHandshakeTimeout:   dialer.opts.TLSHandshakeTimeout,
		ResponseHeaderTimeout: dialer.opts.ResponseHeaderTimeout,
		ForceAttemptHTTP2:     true,
	}

	if opts.ProxyFunc != nil {
		inner.Proxy = opts.ProxyFunc
	}

	return &Transport{
		inner:   inner,
		policy:  opts.Policy,
		headers: opts.Headers,
	}, nil
}

// RoundTrip implements http.RoundTripper.
// It enforces network policy, injects headers, and forwards to the inner transport.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check ACL before making the request.
	if err := t.policy.CheckRequest(req); err != nil {
		return nil, fmt.Errorf("network/transport: policy denied %s: %w", req.URL.Host, err)
	}

	// Clone the request so we can safely modify headers.
	r := req.Clone(req.Context())
	t.headers.Apply(r)

	return t.inner.RoundTrip(r)
}

// NewHTTPClient returns an *http.Client backed by this transport with
// the timeout settings from the dialer options.
func (t *Transport) NewHTTPClient() *http.Client {
	return &http.Client{Transport: t}
}
