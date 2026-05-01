// internal/platform/windows/appcontainer/container.go
//go:build windows

// Package appcontainer provides a stub for Windows AppContainer-style
// process isolation. Full implementation requires the CreateAppContainerProfile
// and CreateProcess with PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES APIs.
// Phase 1: stub that signals intent but does not yet apply AC isolation.
package appcontainer

import "fmt"

// Container represents an AppContainer configuration for a renderer session.
type Container struct {
	Name         string
	SessionID    string
	Capabilities []Capability
}

// New creates a Container descriptor for sessionID.
// The name must be unique per user per machine.
func New(sessionID string, caps []Capability) *Container {
	return &Container{
		Name:         "ghost-silicon-" + sessionID,
		SessionID:    sessionID,
		Capabilities: caps,
	}
}

// Apply creates the AppContainer profile in the OS.
// Phase 1 stub — logs intent and returns nil.
func (c *Container) Apply() error {
	// TODO: call CreateAppContainerProfile via syscall
	fmt.Printf("appcontainer: [stub] would create profile %q with %d capabilities\n",
		c.Name, len(c.Capabilities))
	return nil
}

// Remove deletes the AppContainer profile from the OS.
// Phase 1 stub.
func (c *Container) Remove() error {
	// TODO: call DeleteAppContainerProfile via syscall
	fmt.Printf("appcontainer: [stub] would remove profile %q\n", c.Name)
	return nil
}
