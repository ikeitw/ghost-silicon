// internal/platform/windows/firewall/policy.go
//go:build windows

// Package firewall — firewall policy application.
// Applies a complete set of rules for a session on startup and removes
// them on session stop.
package firewall

import "fmt"

// SessionPolicy holds the full set of rules for one renderer session.
type SessionPolicy struct {
	SessionID string
	Rules     []Rule
}

// DefaultSessionPolicy returns the standard rule set for a renderer session.
func DefaultSessionPolicy(sessionID string) *SessionPolicy {
	return &SessionPolicy{
		SessionID: sessionID,
		Rules: []Rule{
			DefaultOutboundAllowRule(sessionID),
			DefaultInboundBlockRule(sessionID),
		},
	}
}

// Apply installs all rules in the policy.
func (p *SessionPolicy) Apply() error {
	if err := AllowOutbound(0, p.SessionID); err != nil {
		return fmt.Errorf("firewall/policy: apply outbound allow: %w", err)
	}
	if err := BlockLoopbackExpose(p.SessionID); err != nil {
		return fmt.Errorf("firewall/policy: apply inbound block: %w", err)
	}
	return nil
}

// Remove deletes all rules belonging to the session.
func (p *SessionPolicy) Remove() error {
	if err := RemoveRules(p.SessionID); err != nil {
		return fmt.Errorf("firewall/policy: remove rules: %w", err)
	}
	// Also remove the inbound block rule.
	inName := "gs-inbound-block-" + p.SessionID
	_ = removeNamedRule(inName) // best-effort
	return nil
}

// removeNamedRule deletes a rule by exact name.
func removeNamedRule(name string) error {
	return RemoveRules(name)
}
