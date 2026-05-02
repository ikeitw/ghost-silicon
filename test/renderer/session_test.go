// test/renderer/session_test.go
package renderer_test

import (
	"testing"
	"time"

	"ghost-silicon/pkg/renderer"
)

type mockProcess struct{ running bool }

func (m *mockProcess) PID() uint32                                                { return 1234 }
func (m *mockProcess) IsRunning() bool                                            { return m.running }
func (m *mockProcess) Terminate() error                                           { m.running = false; return nil }
func (m *mockProcess) Wait(_ interface{ Done() <-chan struct{} }) (uint32, error) { return 0, nil }

func newTestSession(state renderer.SessionState) *renderer.Session {
	proc := &renderer.MockProcessHandle{PIDVal: 42, Running: true}
	s := renderer.NewSession("test-session-id", "test-profile-id", "/tmp/session", proc)
	s.SetState(state)
	return s
}

func TestSession_InitialState(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("s1", "p1", "/data/s1", proc)
	if s.State() != renderer.StateStarting {
		t.Errorf("expected StateStarting, got %s", s.State())
	}
}

func TestSession_SetState(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("s1", "p1", "/data/s1", proc)
	s.SetState(renderer.StateReady)
	if s.State() != renderer.StateReady {
		t.Errorf("expected StateReady, got %s", s.State())
	}
}

func TestSession_RecordCrash(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("s1", "p1", "/data/s1", proc)
	s.RecordCrash()
	if s.CrashCount() != 1 {
		t.Errorf("expected crash count 1, got %d", s.CrashCount())
	}
	if s.State() != renderer.StateCrashed {
		t.Errorf("expected StateCrashed after RecordCrash, got %s", s.State())
	}
}

func TestSession_IsActive(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("s1", "p1", "/data/s1", proc)
	s.SetState(renderer.StateReady)
	if !s.IsActive() {
		t.Error("expected IsActive=true for StateReady")
	}
	s.SetState(renderer.StateStopped)
	if s.IsActive() {
		t.Error("expected IsActive=false for StateStopped")
	}
}

func TestSession_Uptime(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("s1", "p1", "/data/s1", proc)
	time.Sleep(10 * time.Millisecond)
	uptime := s.Uptime()
	if uptime < 10*time.Millisecond {
		t.Errorf("expected uptime >= 10ms, got %v", uptime)
	}
}

func TestSession_IDs(t *testing.T) {
	proc := renderer.NewMockProcess(42)
	s := renderer.NewSession("my-session", "my-profile", "/data", proc)
	if s.ID() != "my-session" {
		t.Errorf("ID: got %q, want my-session", s.ID())
	}
	if s.ProfileID() != "my-profile" {
		t.Errorf("ProfileID: got %q, want my-profile", s.ProfileID())
	}
	if s.UserDataDir() != "/data" {
		t.Errorf("UserDataDir: got %q, want /data", s.UserDataDir())
	}
}
