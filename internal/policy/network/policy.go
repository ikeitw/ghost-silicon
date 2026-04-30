// internal/policy/network/policy.go
// Package network defines the supervisor-level network policy engine.
// It decides whether outbound connections from the renderer are permitted
// based on rules loaded from the YAML network-policy config file.
package network

import (
	"fmt"
	"net/url"
	"strings"
)

// Decision is the result of a policy evaluation.
type Decision string

const (
	DecisionAllow Decision = "allow"
	DecisionDeny  Decision = "deny"
)

// Rule is a single ACL entry.
type Rule struct {
	// Host is a hostname or wildcard pattern, e.g. "*.example.com".
	Host string `yaml:"host"`
	// Action is "allow" or "deny".
	Action Decision `yaml:"action"`
}

// Policy evaluates network connection requests against an ordered rule list.
// Rules are evaluated top-to-bottom; the first match wins.
// If no rule matches, DefaultAction is applied.
type Policy struct {
	Rules         []Rule   `yaml:"rules"`
	DefaultAction Decision `yaml:"default_action"`
}

// DefaultPolicy returns a permissive policy (allow all, no rules).
func DefaultPolicy() *Policy {
	return &Policy{DefaultAction: DecisionAllow}
}

// Evaluate returns the policy Decision for rawURL.
func (p *Policy) Evaluate(rawURL string) (Decision, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return DecisionDeny, fmt.Errorf("network/policy: invalid URL %q: %w", rawURL, err)
	}
	host := strings.ToLower(u.Hostname())

	for _, r := range p.Rules {
		if matchHost(r.Host, host) {
			return r.Action, nil
		}
	}
	return p.DefaultAction, nil
}

// matchHost returns true when pattern matches host.
// A leading "*." means the pattern matches all subdomains.
func matchHost(pattern, host string) bool {
	pattern = strings.ToLower(pattern)
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return strings.HasSuffix(host, suffix) || host == pattern[2:]
	}
	return pattern == host
}
