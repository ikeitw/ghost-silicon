// pkg/sandbox/windows_firewall.go
//go:build windows

// Package sandbox — Windows Firewall helpers at the pkg/sandbox layer.
// Delegates to internal/platform/windows/firewall for actual rule management.
package sandbox

import (
	"ghost-silicon/internal/platform/windows/firewall"
)

// ApplyFirewallRules applies the standard ghost-silicon outbound firewall
// policy for the renderer process identified by pid.
func ApplyFirewallRules(pid uint32, sessionID string) error {
	return firewall.AllowOutbound(pid, sessionID)
}

// RemoveFirewallRules removes the rules added for sessionID.
func RemoveFirewallRules(sessionID string) error {
	return firewall.RemoveRules(sessionID)
}
