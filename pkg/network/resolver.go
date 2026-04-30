// pkg/network/resolver.go
// Package network — custom DNS resolver.
// Allows the supervisor to route DNS lookups through configured servers
// instead of the system resolver, so per-profile DNS policy is enforced.
package network

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Resolver wraps net.Resolver with optional server override.
type Resolver struct {
	servers []string // e.g. ["1.1.1.1:53", "8.8.8.8:53"]
	inner   *net.Resolver
}

// NewResolver creates a Resolver that uses the given DNS servers.
// If servers is empty the system resolver is used.
func NewResolver(servers []string) *Resolver {
	r := &Resolver{servers: servers}
	if len(servers) == 0 {
		r.inner = net.DefaultResolver
		return r
	}

	// Pick the first server as the primary; net.Resolver dials it per lookup.
	primary := servers[0]
	r.inner = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			conn, err := d.DialContext(ctx, "udp", primary)
			if err != nil {
				return nil, fmt.Errorf("resolver: dial %s: %w", primary, err)
			}
			return conn, nil
		},
	}
	return r
}

// LookupHost resolves host to a list of IP address strings.
func (r *Resolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	addrs, err := r.inner.LookupHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolver: lookup %q: %w", host, err)
	}
	return addrs, nil
}

// netResolver returns the underlying *net.Resolver for use in net.Dialer.
func (r *Resolver) netResolver() *net.Resolver {
	return r.inner
}

// Servers returns the configured DNS server list (may be empty).
func (r *Resolver) Servers() []string { return r.servers }
