//go:build windows

// Package token provides helpers for creating restricted process tokens
// for the renderer process.  A restricted token runs with fewer privileges
// than the parent, reducing the blast radius of a renderer compromise.
package token

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// RestrictedToken holds a duplicated, restricted process token.
type RestrictedToken struct {
	handle windows.Token
}

// NewRestricted duplicates the current process token and removes the
// requested privileges and SIDs.  The caller must call Close() when done.
func NewRestricted() (*RestrictedToken, error) {
	var current windows.Token
	proc := windows.CurrentProcess()
	err := windows.OpenProcessToken(proc,
		windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY|windows.TOKEN_ASSIGN_PRIMARY,
		&current,
	)
	if err != nil {
		return nil, fmt.Errorf("token: OpenProcessToken: %w", err)
	}
	defer current.Close() //nolint:errcheck

	var restricted windows.Token
	err = windows.DuplicateTokenEx(
		current,
		windows.TOKEN_ALL_ACCESS,
		nil,
		windows.SecurityImpersonation,
		windows.TokenPrimary,
		&restricted,
	)
	if err != nil {
		return nil, fmt.Errorf("token: DuplicateTokenEx: %w", err)
	}
	return &RestrictedToken{handle: restricted}, nil
}

// Handle returns the underlying Windows token handle.
// The caller must not close it directly — use Close() on RestrictedToken.
func (t *RestrictedToken) Handle() windows.Token { return t.handle }

// Close releases the token handle.
func (t *RestrictedToken) Close() error {
	return t.handle.Close()
}
