//go:build windows

package filesystem

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// LockToCurrentUser applies a DACL to path that allows only the current user
// (Full Control) and denies Everyone else.
func LockToCurrentUser(path string) error {
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		return fmt.Errorf("filesystem/acl: OpenProcessToken: %w", err)
	}
	defer tok.Close() //nolint:errcheck

	user, err := tok.GetTokenUser()
	if err != nil {
		return fmt.Errorf("filesystem/acl: GetTokenUser: %w", err)
	}

	// Sid.String() returns only a string (no error) in golang.org/x/sys/windows.
	sidStr := user.User.Sid.String()

	sddl := fmt.Sprintf("O:%sG:%sD:(A;OICI;FA;;;%s)", sidStr, sidStr, sidStr)

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("filesystem/acl: parse SDDL: %w", err)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("filesystem/acl: extract DACL: %w", err)
	}

	// SetNamedSecurityInfo takes a string path, not *uint16.
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	)
	if err != nil {
		return fmt.Errorf("filesystem/acl: SetNamedSecurityInfo on %q: %w", path, err)
	}
	return nil
}
