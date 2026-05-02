// test/windows/firewall_test.go
//go:build windows

package windows_test

import (
	"testing"

	"ghost-silicon/internal/platform/windows/firewall"
)

func TestFirewall_DefaultOutboundRule(t *testing.T) {
	rule := firewall.DefaultOutboundAllowRule("test-session-fw-001")
	if rule.Name == "" {
		t.Error("rule name must not be empty")
	}
	if rule.Direction != firewall.DirectionOut {
		t.Errorf("expected direction out, got %s", rule.Direction)
	}
	if rule.Action != firewall.ActionAllow {
		t.Errorf("expected action allow, got %s", rule.Action)
	}
}

func TestFirewall_DefaultInboundBlockRule(t *testing.T) {
	rule := firewall.DefaultInboundBlockRule("test-session-fw-002")
	if rule.Direction != firewall.DirectionIn {
		t.Errorf("expected direction in, got %s", rule.Direction)
	}
	if rule.Action != firewall.ActionBlock {
		t.Errorf("expected action block, got %s", rule.Action)
	}
}

func TestFirewall_SessionPolicy_DefaultRules(t *testing.T) {
	policy := firewall.DefaultSessionPolicy("test-session-fw-003")
	if policy.SessionID != "test-session-fw-003" {
		t.Errorf("session ID mismatch: %s", policy.SessionID)
	}
	if len(policy.Rules) == 0 {
		t.Error("expected at least one rule in default policy")
	}
}
