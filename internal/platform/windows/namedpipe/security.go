//go:build windows

package namedpipe

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// CurrentUserSecurityAttributes returns a SECURITY_ATTRIBUTES that restricts
// the named pipe so only the current user (owner) can connect.
//
// This prevents other users on the same machine from connecting to the bridge
// pipe and injecting spoofed profile responses.
func CurrentUserSecurityAttributes() (*windows.SecurityAttributes, error) {
	// Get the current user's SID.
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		return nil, fmt.Errorf("namedpipe/security: OpenProcessToken: %w", err)
	}
	defer tok.Close() //nolint:errcheck

	user, err := tok.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("namedpipe/security: GetTokenUser: %w", err)
	}

	// Build a DACL that grants GENERIC_ALL to the current user only.
	// SDDL: D:(A;;GA;;;<SID>)
	sidStr, err := user.User.Sid.String()
	if err != nil {
		return nil, fmt.Errorf("namedpipe/security: SID to string: %w", err)
	}

	sddl := fmt.Sprintf("D:(A;;GA;;;%s)", sidStr)
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("namedpipe/security: build SDDL %q: %w", sddl, err)
	}

	sa := &windows.SecurityAttributes{
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}
	sa.Length = uint32(12)
	return sa, nil
}
