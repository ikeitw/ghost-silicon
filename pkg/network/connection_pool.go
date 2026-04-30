// pkg/network/connection_pool.go
// Package network — connection pool configuration.
// Controls how many idle TCP connections are kept alive in the transport pool.
package network

import "time"

// ConnectionPool holds keep-alive connection pool settings.
type ConnectionPool struct {
	// MaxIdleConns is the total number of idle connections across all hosts.
	MaxIdleConns int

	// MaxIdleConnsPerHost is the per-host idle connection cap.
	MaxIdleConnsPerHost int

	// MaxConnsPerHost is the total connection cap per host (0 = no cap).
	MaxConnsPerHost int

	// IdleConnTimeout is how long an idle connection stays in the pool.
	IdleConnTimeout time.Duration
}

// DefaultConnectionPool returns sensible defaults for a lightly loaded
// single-session supervisor.
func DefaultConnectionPool() *ConnectionPool {
	return &ConnectionPool{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     90 * time.Second,
	}
}

// FromConfig builds a ConnectionPool from the config values.
func FromConfig(maxIdlePerHost int, idleTimeout time.Duration) *ConnectionPool {
	p := DefaultConnectionPool()
	if maxIdlePerHost > 0 {
		p.MaxIdleConnsPerHost = maxIdlePerHost
	}
	if idleTimeout > 0 {
		p.IdleConnTimeout = idleTimeout
	}
	return p
}
