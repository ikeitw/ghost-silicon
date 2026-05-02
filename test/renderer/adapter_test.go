// test/renderer/adapter_test.go
package renderer_test

import (
	"context"
	"testing"

	"ghost-silicon/internal/engine/adapter"
	"ghost-silicon/pkg/renderer"
)

func TestMockAdapter_Name(t *testing.T) {
	a := adapter.NewMockAdapter()
	if a.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", a.Name())
	}
}

func TestMockAdapter_Start_ValidOptions(t *testing.T) {
	a := adapter.NewMockAdapter()
	opts := renderer.StartOptions{
		SessionID:   "test-session",
		ProfileID:   "test-profile",
		PipeName:    `\\.\pipe\gs-test`,
		UserDataDir: `C:\tmp\session`,
	}
	proc, err := a.Start(context.Background(), opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if proc == nil {
		t.Fatal("Start returned nil process")
	}
}

func TestMockAdapter_Start_MissingSessionID(t *testing.T) {
	a := adapter.NewMockAdapter()
	opts := renderer.StartOptions{
		ProfileID:   "test-profile",
		PipeName:    `\\.\pipe\gs-test`,
		UserDataDir: `C:\tmp\session`,
	}
	_, err := a.Start(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing SessionID")
	}
}

func TestMockAdapter_Start_MissingPipeName(t *testing.T) {
	a := adapter.NewMockAdapter()
	opts := renderer.StartOptions{
		SessionID:   "test-session",
		ProfileID:   "test-profile",
		UserDataDir: `C:\tmp\session`,
	}
	_, err := a.Start(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing PipeName")
	}
}

func TestMockProcess_IsRunning(t *testing.T) {
	a := adapter.NewMockAdapter()
	opts := renderer.StartOptions{
		SessionID:   "s1",
		ProfileID:   "p1",
		PipeName:    `\\.\pipe\gs-test`,
		UserDataDir: `C:\tmp\s1`,
	}
	proc, _ := a.Start(context.Background(), opts)
	if !proc.IsRunning() {
		t.Error("mock process should report IsRunning=true immediately after Start")
	}
}

func TestMockProcess_Terminate(t *testing.T) {
	a := adapter.NewMockAdapter()
	opts := renderer.StartOptions{
		SessionID:   "s1",
		ProfileID:   "p1",
		PipeName:    `\\.\pipe\gs-test`,
		UserDataDir: `C:\tmp\s1`,
	}
	proc, _ := a.Start(context.Background(), opts)
	if err := proc.Terminate(); err != nil {
		t.Fatalf("Terminate: %v", err)
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	_, err := adapter.Global.Get("does-not-exist", "")
	if err == nil {
		t.Fatal("expected error for unknown adapter name")
	}
}

func TestRegistry_GetMock(t *testing.T) {
	a, err := adapter.Global.Get("mock", "")
	if err != nil {
		t.Fatalf("Get mock: %v", err)
	}
	if a.Name() != "mock" {
		t.Errorf("expected name 'mock', got %q", a.Name())
	}
}
