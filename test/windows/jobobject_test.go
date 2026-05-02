// test/windows/jobobject_test.go
//go:build windows

package windows_test

import (
	"testing"

	"ghost-silicon/internal/platform/windows/jobobject"
)

func TestJobObject_CreateAndClose(t *testing.T) {
	jo, err := jobobject.Create("ghost-silicon-test-job-1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := jo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestJobObject_SetMemoryLimit(t *testing.T) {
	jo, err := jobobject.Create("ghost-silicon-test-job-2")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer jo.Close() //nolint:errcheck

	if err := jo.SetMemoryLimit(512); err != nil {
		t.Fatalf("SetMemoryLimit: %v", err)
	}
}

func TestJobObject_SetMemoryLimit_Zero(t *testing.T) {
	jo, err := jobobject.Create("ghost-silicon-test-job-3")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer jo.Close() //nolint:errcheck

	// Zero should be a no-op, not an error.
	if err := jo.SetMemoryLimit(0); err != nil {
		t.Fatalf("SetMemoryLimit(0): %v", err)
	}
}

func TestJobObject_Limits_Apply(t *testing.T) {
	jo, err := jobobject.Create("ghost-silicon-test-job-4")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer jo.Close() //nolint:errcheck

	limits := &jobobject.Limits{
		MemoryLimitMB:  1024,
		CPURatePercent: 0, // skip CPU rate — requires Windows 8+
	}
	if err := limits.Apply(jo); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

func TestJobObject_Name(t *testing.T) {
	const name = "ghost-silicon-test-job-name"
	jo, err := jobobject.Create(name)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer jo.Close() //nolint:errcheck

	if jo.Name() != name {
		t.Errorf("Name: got %q, want %q", jo.Name(), name)
	}
}
