//go:build windows

package token

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// privilegesToRemove is the list of privileges stripped from the renderer
// token.  These are capabilities that a browser renderer process should never
// need and that could be abused if the renderer is compromised.
var privilegesToRemove = []string{
	"SeDebugPrivilege",         // attach debugger to arbitrary processes
	"SeLoadDriverPrivilege",    // load kernel drivers
	"SeTcbPrivilege",           // act as part of the OS
	"SeBackupPrivilege",        // bypass file ACLs for backup
	"SeRestorePrivilege",       // bypass file ACLs for restore
	"SeCreateTokenPrivilege",   // create arbitrary tokens
	"SeTakeOwnershipPrivilege", // take ownership of any object
	"SeAssignPrimaryTokenPrivilege",
	"SeImpersonatePrivilege",  // impersonate clients (needed only by services)
	"SeCreateGlobalPrivilege", // create global kernel objects in session 0
}

// RemovePrivileges strips the well-known dangerous privileges from tok.
// Privileges that are not present on the token are silently skipped.
func RemovePrivileges(tok *RestrictedToken) error {
	for _, name := range privilegesToRemove {
		if err := removePrivilege(tok.handle, name); err != nil {
			return fmt.Errorf("token/privileges: remove %s: %w", name, err)
		}
	}
	return nil
}

func removePrivilege(tok windows.Token, name string) error {
	var luid windows.LUID
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	if err := windows.LookupPrivilegeValue(nil, namePtr, &luid); err != nil {
		// Privilege not available on this system — skip.
		return nil
	}

	tp := windows.Tokenprivileges{
		PrivilegeCount: 1,
	}
	tp.Privileges[0] = windows.LUIDAndAttributes{
		Luid:       luid,
		Attributes: windows.SE_PRIVILEGE_REMOVED,
	}

	return windows.AdjustTokenPrivileges(tok, false, &tp, uint32(unsafe.Sizeof(tp)), nil, nil)
}
