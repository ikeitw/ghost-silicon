// pkg/network/policy.go
// Package network — network ACL policy.
// Decides whether an outbound HTTP request is permitted based on host
// and port allowlists/blocklists loaded from configuration.
package network

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Policy is the network access-control policy for a session.
// An empty policy allows everything.
type Policy struct {
	allowedHosts map[string]struct{}
	blockedHosts map[string]struct{}
	allowedPorts map[int]struct{}
}

// DefaultPolicy returns a permissive policy (allow all).
func DefaultPolicy() *Policy { return &Policy{} }

// NewPolicy builds a Policy from the config ACL lists.
func NewPolicy(allowedHosts, blockedHosts []string, allowedPorts []int) *Policy {
	p := &Policy{
		allowedHosts: toSet(allowedHosts),
		blockedHosts: toSet(blockedHosts),
		allowedPorts: toIntSet(allowedPorts),
	}
	return p
}

// CheckRequest returns nil when req is permitted, or an error describing
// why it is blocked.
func (p *Policy) CheckRequest(req *http.Request) error {
	host := req.URL.Hostname()
	portStr := req.URL.Port()

	// Check blocked hosts first.
	if p.isBlocked(host) {
		return fmt.Errorf("host %q is blocked by network policy", host)
	}

	// If allowedHosts is non-empty, the host must be in it.
	if len(p.allowedHosts) > 0 && !p.isAllowed(host) {
		return fmt.Errorf("host %q is not in the network policy allowlist", host)
	}

	// Port check.
	if len(p.allowedPorts) > 0 && portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("invalid port %q in request URL", portStr)
		}
		if _, ok := p.allowedPorts[port]; !ok {
			return fmt.Errorf("port %d is not in the network policy allowlist", port)
		}
	}

	return nil
}

func (p *Policy) isBlocked(host string) bool {
	if p.blockedHosts == nil {
		return false
	}
	host = strings.ToLower(host)
	if _, ok := p.blockedHosts[host]; ok {
		return true
	}
	// Wildcard suffix match: *.example.com blocks sub.example.com
	for blocked := range p.blockedHosts {
		if strings.HasPrefix(blocked, "*.") {
			suffix := blocked[1:] // ".example.com"
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

func (p *Policy) isAllowed(host string) bool {
	host = strings.ToLower(host)
	if _, ok := p.allowedHosts[host]; ok {
		return true
	}
	for allowed := range p.allowedHosts {
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:]
			if strings.HasSuffix(host, suffix) {
				return true
			}
		}
	}
	return false
}

func toSet(ss []string) map[string]struct{} {
	if len(ss) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[strings.ToLower(strings.TrimSpace(s))] = struct{}{}
	}
	return m
}

func toIntSet(ns []int) map[int]struct{} {
	if len(ns) == 0 {
		return nil
	}
	m := make(map[int]struct{}, len(ns))
	for _, n := range ns {
		m[n] = struct{}{}
	}
	return m
}
