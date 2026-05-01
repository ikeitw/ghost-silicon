// pkg/sandbox/windows_token.go
//go:build windows

// Package sandbox — Windows token helpers at the pkg/sandbox layer.
// Convenience constructors that build restricted tokens from sandbox Options.
package sandbox

import (
	"fmt"

	"ghost-silicon/internal/platform/windows/token"
)

// BuildToken constructs a restricted process token from sandbox Options.
// Returns nil, nil when restricted tokens are not requested.
func BuildToken(integrityLevel string) (*token.RestrictedToken, error) {
	level, err := token.ParseIntegrityLevel(integrityLevel)
	if err != nil {
		return nil, fmt.Errorf("sandbox/token: %w", err)
	}
	tok, err := token.Build(token.Config{
		IntegrityLevel:   level,
		RemovePrivileges: true,
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox/token: build: %w", err)
	}
	return tok, nil
}
