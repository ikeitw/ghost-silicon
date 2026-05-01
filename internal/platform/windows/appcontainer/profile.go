// internal/platform/windows/appcontainer/profile.go
//go:build windows

// Package appcontainer — AppContainer profile descriptor.
package appcontainer

// Profile describes the security profile applied to an AppContainer.
type Profile struct {
	// Name is the unique AppContainer profile name.
	Name string

	// DisplayName is a human-readable label shown in Task Manager.
	DisplayName string

	// Description describes the container purpose.
	Description string

	// Capabilities is the list of capabilities granted to the container.
	Capabilities []Capability
}

// DefaultRendererProfile returns a minimal AppContainer profile for the
// renderer process with only the capabilities needed for web browsing.
func DefaultRendererProfile(sessionID string) *Profile {
	return &Profile{
		Name:        "ghost-silicon-" + sessionID,
		DisplayName: "Ghost Silicon Renderer",
		Description: "Isolated renderer session " + sessionID,
		Capabilities: []Capability{
			CapInternetClient,
			CapPrivateNetworkClientServer,
		},
	}
}
