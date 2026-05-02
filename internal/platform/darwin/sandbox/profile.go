// internal/platform/darwin/sandbox/profile.go
//go:build darwin

// Package sandbox — macOS sandbox profile descriptor.
// A Profile is a named sandbox-exec policy written in the Seatbelt
// profile language. Phase 1 only ships the DefaultProfile stub.
package sandbox

// Profile describes a macOS sandbox-exec policy.
type Profile struct {
	// Name is a human-readable identifier.
	Name string

	// Rules is the Seatbelt profile text passed to sandbox-exec -p.
	// Phase 1: empty. Phase 2 will populate with deny-default rules.
	Rules string
}

// DefaultProfile returns a minimal Seatbelt profile for the renderer.
// Phase 1: allows everything (no restrictions applied).
// Phase 2 will replace this with a deny-default policy.
func DefaultProfile(sessionID string) *Profile {
	return &Profile{
		Name: "ghost-silicon-" + sessionID,
		// Phase 2 rules will look like:
		// (version 1)
		// (deny default)
		// (allow network-outbound)
		// (allow file-read* (subpath "/usr"))
		// ...
		Rules: "(version 1)(allow default)",
	}
}

// HardenedProfile returns a stricter Seatbelt profile stub.
func HardenedProfile(sessionID string) *Profile {
	return &Profile{
		Name:  "ghost-silicon-hardened-" + sessionID,
		Rules: "(version 1)(allow default)", // Phase 1 stub
	}
}
