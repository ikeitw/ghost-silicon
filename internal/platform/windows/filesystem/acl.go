//go:build windows

package filesystem

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// LockToCurrentUser applies a DACL to path that allows only the current user
// (Full Control) and denies Everyone else.  Should be called on the session
// root after CreateSessionLayout.
func LockToCurrentUser(path string) error {
	// Resolve current user SID.
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &tok); err != nil {
		return fmt.Errorf("filesystem/acl: OpenProcessToken: %w", err)
	}
	defer tok.Close() //nolint:errcheck

	user, err := tok.GetTokenUser()
	if err != nil {
		return fmt.Errorf("filesystem/acl: GetTokenUser: %w", err)
	}
	sidStr, err := user.User.Sid.String()
	if err != nil {
		return fmt.Errorf("filesystem/acl: SID string: %w", err)
	}

	// SDDL: owner inherits full control; no other ACEs.
	// O:<SID>G:<SID>D:(A;OICI;FA;;;<SID>)
	sddl := fmt.Sprintf("O:%sG:%sD:(A;OICI;FA;;;%s)", sidStr, sidStr, sidStr)

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("filesystem/acl: parse SDDL: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("filesystem/acl: encode path: %w", err)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("filesystem/acl: extract DACL: %w", err)
	}

	err = windows.SetNamedSecurityInfo(
		pathPtr,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	)
	if err != nil {
		return fmt.Errorf("filesystem/acl: SetNamedSecurityInfo on %q: %w", path, err)
	}
	return nil
}
