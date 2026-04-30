// internal/engine/adapter/adapter.go
// Package adapter defines the RendererAdapter interface and the base
// adapter type that every concrete engine backend embeds.
package adapter

import (
	"context"
	"fmt"

	"ghost-silicon/pkg/renderer"
)

// Base provides shared fields and helpers that every concrete adapter embeds.
// Adapters are not safe for concurrent use — create one per session.
type Base struct {
	name       string
	executable string
}

// NewBase creates a Base for the given engine name and binary path.
func NewBase(name, executable string) Base {
	return Base{name: name, executable: executable}
}

// Name returns the adapter identifier, e.g. "chromium" or "mock".
func (b *Base) Name() string { return b.name }

// Executable returns the path to the renderer binary.
func (b *Base) Executable() string { return b.executable }

// ValidateOptions returns an error when opts is missing required fields.
func (b *Base) ValidateOptions(opts renderer.StartOptions) error {
	if opts.SessionID == "" {
		return fmt.Errorf("adapter[%s]: StartOptions.SessionID must not be empty", b.name)
	}
	if opts.ProfileID == "" {
		return fmt.Errorf("adapter[%s]: StartOptions.ProfileID must not be empty", b.name)
	}
	if opts.PipeName == "" {
		return fmt.Errorf("adapter[%s]: StartOptions.PipeName must not be empty", b.name)
	}
	if opts.UserDataDir == "" {
		return fmt.Errorf("adapter[%s]: StartOptions.UserDataDir must not be empty", b.name)
	}
	return nil
}

// MockAdapter is a no-op adapter used in tests and developer tooling.
// It satisfies renderer.Adapter without launching a real process.
type MockAdapter struct {
	Base
}

// NewMockAdapter creates a MockAdapter.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{Base: NewBase("mock", "")}
}

// Start returns a MockProcess that is immediately "running".
func (m *MockAdapter) Start(_ context.Context, opts renderer.StartOptions) (renderer.Process, error) {
	if err := m.ValidateOptions(opts); err != nil {
		return nil, err
	}
	return &MockProcess{pid: 0}, nil
}

// MockProcess satisfies renderer.Process with no-op behaviour.
type MockProcess struct{ pid uint32 }

func (p *MockProcess) PID() uint32                            { return p.pid }
func (p *MockProcess) IsRunning() bool                        { return true }
func (p *MockProcess) Terminate() error                       { return nil }
func (p *MockProcess) Wait(_ context.Context) (uint32, error) { return 0, nil }
