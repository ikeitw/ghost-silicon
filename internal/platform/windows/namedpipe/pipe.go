//go:build windows

package namedpipe

import "golang.org/x/sys/windows"

// isValidPipeName reports whether name is a syntactically valid Windows named
// pipe path of the form \\.\pipe\<suffix>.
func isValidPipeName(name string) bool {
	const prefix = `\\.\pipe\`
	if len(name) <= len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if name[i] != prefix[i] {
			return false
		}
	}
	return true
}

// setHandleNonInheritable marks a handle so it is not inherited by child
// processes. This is called on the server's listen handle to prevent the
// renderer from accidentally inheriting it.
func setHandleNonInheritable(h windows.Handle) error {
	return windows.SetHandleInformation(h, windows.HANDLE_FLAG_INHERIT, 0)
}
