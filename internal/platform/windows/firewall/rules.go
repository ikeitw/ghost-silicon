// internal/platform/windows/firewall/rules.go
//go:build windows

// Package firewall — firewall rule descriptors.
// Defines the Rule struct used to describe a Windows Firewall rule before
// it is applied via netsh or the COM API.
package firewall

// Direction is the traffic direction for a firewall rule.
type Direction string

const (
	DirectionIn  Direction = "in"
	DirectionOut Direction = "out"
)

// Action is the firewall action for a rule.
type Action string

const (
	ActionAllow Action = "allow"
	ActionBlock Action = "block"
)

// Rule describes a single Windows Firewall rule.
type Rule struct {
	Name        string
	Direction   Direction
	Action      Action
	Protocol    string // "tcp", "udp", "any"
	LocalIP     string // CIDR or "any"
	RemoteIP    string // CIDR, "any", "!LocalSubnet"
	LocalPort   string // port number or "any"
	RemotePort  string // port number or "any"
	Enabled     bool
	Description string
}

// DefaultOutboundAllowRule returns a broad outbound allow rule for sessionID.
func DefaultOutboundAllowRule(sessionID string) Rule {
	return Rule{
		Name:        ruleName(sessionID),
		Direction:   DirectionOut,
		Action:      ActionAllow,
		Protocol:    "any",
		LocalIP:     "any",
		RemoteIP:    "any",
		Enabled:     true,
		Description: "ghost-silicon outbound allow for session " + sessionID,
	}
}

// DefaultInboundBlockRule returns an inbound block rule for all non-loopback
// traffic directed at the renderer process.
func DefaultInboundBlockRule(sessionID string) Rule {
	return Rule{
		Name:        "gs-inbound-block-" + sessionID,
		Direction:   DirectionIn,
		Action:      ActionBlock,
		Protocol:    "any",
		RemoteIP:    "!LocalSubnet",
		Enabled:     true,
		Description: "ghost-silicon inbound block for session " + sessionID,
	}
}
