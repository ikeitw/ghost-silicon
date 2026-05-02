// test/windows/process_test.go
//go:build windows

// Package windows_test contains Windows-specific integration tests for
// the platform isolation layer. These tests require a real Windows 11
// environment and will be skipped on other platforms.
package windows_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"ghost-silicon/internal/platform/windows/process"
)

func TestLaunch_SimpleProcess(t *testing.T) {
	// Launch a simple Windows process that exits immediately.
	opts := process.LaunchOptions{
		Executable: `C:\Windows\System32\cmd.exe`,
		Args:       []string{"/C", "exit 0"},
	}

	proc, err := process.Launch(opts)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer proc.Close() //nolint:errcheck

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exitCode, err := proc.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
}

func TestLaunch_IsRunning(t *testing.T) {
	// Launch a process that sleeps briefly.
	opts := process.LaunchOptions{
		Executable: `C:\Windows\System32\cmd.exe`,
		Args:       []string{"/C", "ping -n 3 127.0.0.1 > nul"},
	}

	proc, err := process.Launch(opts)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer proc.Close() //nolint:errcheck

	if !proc.IsRunning() {
		t.Error("expected IsRunning=true immediately after launch")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, _ = proc.Wait(ctx)
}

func TestLaunch_Terminate(t *testing.T) {
	opts := process.LaunchOptions{
		Executable: `C:\Windows\System32\cmd.exe`,
		Args:       []string{"/C", "ping -n 100 127.0.0.1 > nul"},
	}

	proc, err := process.Launch(opts)
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	defer proc.Close() //nolint:errcheck

	if err := proc.Terminate(1); err != nil {
		t.Fatalf("Terminate: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = proc.Wait(ctx)

	if proc.IsRunning() {
		t.Error("expected IsRunning=false after Terminate")
	}
}

func TestEnvBuilder_SetAndBuild(t *testing.T) {
	b := process.NewEnvBuilder()
	b.Set("TEST_KEY", "test_value")
	b.Remove("TEMP") // strip common Windows temp var

	env := b.Build()
	found := false
	for _, kv := range env {
		if kv == "TEST_KEY=test_value" {
			found = true
		}
		if len(kv) > 5 && kv[:5] == "TEMP=" {
			t.Error("TEMP variable should have been removed")
		}
	}
	if !found {
		t.Error("TEST_KEY=test_value not found in built environment")
	}
}

func TestEnvBuilder_SetSession(t *testing.T) {
	b := process.NewEnvBuilder()
	b.SetSession("sess-123", "prof-456", `\\.\pipe\gs-test`)

	env := b.Build()
	want := map[string]string{
		"GS_SESSION_ID": "sess-123",
		"GS_PROFILE_ID": "prof-456",
		"GS_PIPE_NAME":  `\\.\pipe\gs-test`,
	}
	for k, v := range want {
		found := false
		for _, kv := range env {
			if kv == k+"="+v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("env missing %s=%s", k, v)
		}
	}
}

// TestCmdAvailable is a sanity check that cmd.exe is on the path.
func TestCmdAvailable(t *testing.T) {
	_, err := exec.LookPath("cmd.exe")
	if err != nil {
		t.Skip("cmd.exe not found — skipping Windows process tests")
	}
}
