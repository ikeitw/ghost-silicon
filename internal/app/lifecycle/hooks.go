// internal/app/lifecycle/hooks.go
// Package lifecycle — pre-built hook constructors for common subsystems.
// Each function returns a named Hook pair (start + stop) so the caller
// registers them consistently without duplicating the name strings.
package lifecycle

import (
	"context"
	"net"
)

// HookPair bundles a startup and shutdown hook under one name.
type HookPair struct {
	Name  string
	Start func(ctx context.Context) error
	Stop  func(ctx context.Context) error
}

// Register adds both hooks of a HookPair to the Manager.
func (m *Manager) Register(p HookPair) {
	m.OnStart(p.Name, p.Start)
	m.OnStop(p.Name, p.Stop)
}

// ListenerHook creates a HookPair for a net.Listener-based server.
// onStart is called with the listener; onStop closes it.
func ListenerHook(name string, ln net.Listener, serve func(context.Context, net.Listener) error) HookPair {
	return HookPair{
		Name: name,
		Start: func(ctx context.Context) error {
			go func() { _ = serve(ctx, ln) }()
			return nil
		},
		Stop: func(_ context.Context) error {
			return ln.Close()
		},
	}
}

// FuncHook creates a HookPair from plain functions.
func FuncHook(
	name string,
	start func(context.Context) error,
	stop func(context.Context) error,
) HookPair {
	return HookPair{Name: name, Start: start, Stop: stop}
}
