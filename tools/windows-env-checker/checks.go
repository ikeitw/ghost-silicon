// tools/windows-env-checker/checks.go
// Environment check implementations.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Checks holds all environment check functions.
type Checks struct{}

// NewChecks creates a Checks instance.
func NewChecks() *Checks { return &Checks{} }

// RunAll executes every check and returns an EnvReport.
func (c *Checks) RunAll() *EnvReport {
	r := &EnvReport{}

	r.Add(c.checkOS())
	r.Add(c.checkGoVersion())
	r.Add(c.checkAppData())
	r.Add(c.checkWindowsVersion())
	r.Add(c.checkGitAvailable())

	return r
}

// checkOS verifies the OS is Windows.
func (c *Checks) checkOS() EnvCheck {
	ok := runtime.GOOS == "windows"
	detail := runtime.GOOS + "/" + runtime.GOARCH
	msg := ""
	if !ok {
		msg = "ghost-silicon requires Windows 11 for full isolation support"
	}
	return EnvCheck{
		Name:    "operating-system",
		Value:   detail,
		Passed:  ok,
		Message: msg,
	}
}

// checkGoVersion verifies Go 1.22+.
func (c *Checks) checkGoVersion() EnvCheck {
	ver := runtime.Version() // e.g. "go1.22.4"
	ok := strings.HasPrefix(ver, "go1.22") ||
		strings.HasPrefix(ver, "go1.23") ||
		strings.HasPrefix(ver, "go1.24")
	msg := ""
	if !ok {
		msg = "Go 1.22 or later is required"
	}
	return EnvCheck{
		Name:    "go-version",
		Value:   ver,
		Passed:  ok,
		Message: msg,
	}
}

// checkAppData verifies %APPDATA% is set and accessible.
func (c *Checks) checkAppData() EnvCheck {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return EnvCheck{
			Name:    "APPDATA",
			Value:   "(not set)",
			Passed:  false,
			Message: "%APPDATA% environment variable is not set",
		}
	}
	info, err := os.Stat(appData)
	ok := err == nil && info.IsDir()
	msg := ""
	if !ok {
		msg = fmt.Sprintf("cannot access APPDATA directory: %v", err)
	}
	return EnvCheck{
		Name:    "APPDATA",
		Value:   appData,
		Passed:  ok,
		Message: msg,
	}
}

// checkWindowsVersion reads the OS version from the registry on Windows,
// or reports a stub on non-Windows platforms.
func (c *Checks) checkWindowsVersion() EnvCheck {
	if runtime.GOOS != "windows" {
		return EnvCheck{
			Name:    "windows-version",
			Value:   "n/a (not Windows)",
			Passed:  true,
			Message: "",
		}
	}
	// Query the registry for the Windows build number.
	out, err := exec.Command("reg", "query",
		`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`,
		"/v", "CurrentBuild").Output()
	if err != nil {
		return EnvCheck{
			Name:    "windows-version",
			Value:   "unknown",
			Passed:  false,
			Message: "could not read Windows version from registry",
		}
	}
	// Parse build number from output.
	lines := strings.Split(string(out), "\n")
	build := "unknown"
	for _, line := range lines {
		if strings.Contains(line, "CurrentBuild") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				build = parts[len(parts)-1]
			}
		}
	}
	// Windows 11 starts at build 22000.
	ok := buildNumberOK(build)
	msg := ""
	if !ok {
		msg = "Windows 11 (build 22000+) is recommended for full Job Object support"
	}
	return EnvCheck{
		Name:    "windows-version",
		Value:   "build " + strings.TrimSpace(build),
		Passed:  ok,
		Message: msg,
	}
}

// checkGitAvailable checks that git is on PATH (useful for dev builds).
func (c *Checks) checkGitAvailable() EnvCheck {
	path, err := exec.LookPath("git")
	ok := err == nil
	val := path
	msg := ""
	if !ok {
		val = "not found"
		msg = "git is not on PATH — needed for build version injection"
	}
	return EnvCheck{
		Name:    "git",
		Value:   val,
		Passed:  ok,
		Message: msg,
	}
}

// buildNumberOK returns true when the build number string represents
// Windows 11 (22000) or later.
func buildNumberOK(build string) bool {
	build = strings.TrimSpace(build)
	var n int
	_, err := fmt.Sscanf(build, "%d", &n)
	if err != nil {
		return false
	}
	return n >= 22000
}
