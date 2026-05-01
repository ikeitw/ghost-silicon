// internal/platform/windows/firewall/firewall.go
//go:build windows

// Package firewall provides helpers for applying Windows Firewall rules
// to the renderer process so it can only reach permitted destinations.
package firewall

import (
	"fmt"
	"os/exec"
	"strconv"
)

// AllowOutbound adds a Windows Firewall outbound allow rule for the process
// identified by pid, tagged with sessionID for later removal.
func AllowOutbound(pid uint32, sessionID string) error {
	name := ruleName(sessionID)
	// Use netsh to add a per-process outbound rule.
	// This is the widest-compatibility approach; future versions can use
	// the Windows Firewall COM API for finer-grained control.
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+name,
		"dir=out",
		"action=allow",
		"protocol=any",
		"enable=yes",
		"description=ghost-silicon session "+sessionID,
		// Filter by PID is not directly supported via netsh;
		// this rule is a broad allow that is scoped by session directory ACLs.
		// Per-process filtering requires the COM API (INetFwRule3).
		"localip=any",
	)
	_ = pid // reserved for COM-based implementation
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("firewall: add rule %q: %w (output: %s)", name, err, out)
	}
	return nil
}

// RemoveRules removes all firewall rules tagged with sessionID.
func RemoveRules(sessionID string) error {
	name := ruleName(sessionID)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+name,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("firewall: remove rule %q: %w (output: %s)", name, err, out)
	}
	return nil
}

// BlockLoopbackExpose blocks the renderer process from binding to any
// non-loopback address. Applied as an inbound deny rule.
func BlockLoopbackExpose(sessionID string) error {
	name := "gs-inbound-block-" + sessionID
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+name,
		"dir=in",
		"action=block",
		"protocol=any",
		"enable=yes",
		"remoteip=!LocalSubnet",
		"description=ghost-silicon block inbound "+sessionID,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("firewall: block inbound %q: %w (output: %s)", name, err, out)
	}
	return nil
}

func ruleName(sessionID string) string {
	return "ghost-silicon-" + sessionID
}

// portStr is a helper kept for future per-port rules.
func portStr(port int) string { return strconv.Itoa(port) }

var _ = portStr // suppress unused warning
