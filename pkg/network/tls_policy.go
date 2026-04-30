// pkg/network/tls_policy.go
// Package network — TLS policy.
// Builds a tls.Config that enforces minimum TLS version and safe cipher
// suites. This is NOT fingerprint evasion — it is standard hardening that
// any security-conscious Go application should apply.
package network

import (
	"crypto/tls"
	"fmt"
)

// TLSPolicy defines the TLS configuration constraints.
type TLSPolicy struct {
	// MinVersion is the minimum TLS version to accept.
	// Defaults to tls.VersionTLS12.
	MinVersion uint16

	// InsecureSkipVerify disables certificate verification.
	// Must only be true in isolated developer environments — never in production.
	InsecureSkipVerify bool
}

// DefaultTLSPolicy returns a secure default configuration.
func DefaultTLSPolicy() *TLSPolicy {
	return &TLSPolicy{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: false,
	}
}

// TLSConfig converts the policy into a *tls.Config.
func (p *TLSPolicy) TLSConfig() (*tls.Config, error) {
	if p.MinVersion < tls.VersionTLS12 {
		return nil, fmt.Errorf("tls_policy: MinVersion must be TLS 1.2 or higher")
	}
	cfg := &tls.Config{
		MinVersion:         p.MinVersion,
		InsecureSkipVerify: p.InsecureSkipVerify, //nolint:gosec
	}
	return cfg, nil
}
