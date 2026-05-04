// test/bridge/bridge_test.go
package bridge_test

import (
	"context"
	"encoding/json"
	"testing"

	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/ipc/messages"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/identity"
)

func newTestBridge(t *testing.T) (*bridge.Bridge, *jsonrpc.Server) {
	t.Helper()
	log := logging.Nop()
	aud := audit.New(log)
	p := identity.Windows11DesktopTemplate()
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "test-session", log, aud)
	br.Register(srv)
	return br, srv
}

func TestBridge_ProfileUpdated(t *testing.T) {
	log := logging.Nop()
	aud := audit.New(log)
	p := identity.Windows11DesktopTemplate()
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "test-session", log, aud)
	br.Register(srv)
	newProfile := identity.Windows11LaptopTemplate()
	br.UpdateProfile(newProfile)
	if br.Profile() == nil {
		t.Fatal("Profile() returned nil after UpdateProfile")
	}
}

func TestBridge_RegistersAllMethods(t *testing.T) {
	_, srv := newTestBridge(t)
	expectedMethods := []string{
		messages.MethodHardwareGetCPUCores,
		messages.MethodHardwareGetRAM,
		messages.MethodHardwareGetGPU,
		messages.MethodNavigatorGetProfile,
		messages.MethodScreenGetProfile,
		messages.MethodNoiseGetCanvasSeed,
		messages.MethodNoiseGetAudioSeed,
		messages.MethodNoiseGetWebGLSeed,
		messages.MethodNoiseGetFontSeed,
		messages.MethodStorageGetPolicy,
		messages.MethodPermissionsGetState,
		messages.MethodSessionGetInfo,
		messages.MethodNetworkGetProfile,
		messages.MethodRendererEvent,
	}
	for _, m := range expectedMethods {
		if !srv.IsRegistered(m) {
			t.Errorf("method %q not registered on bridge", m)
		}
	}
}

func TestBridge_HandleGetSessionInfo(t *testing.T) {
	log := logging.Nop()
	aud := audit.New(log)
	p := identity.Windows11DesktopTemplate()
	const wantSession = "my-test-session"
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, wantSession, log, aud)
	br.Register(srv)
	result, err := srv.Call(context.Background(), messages.MethodSessionGetInfo, nil)
	if err != nil {
		t.Fatalf("Call %s: %v", messages.MethodSessionGetInfo, err)
	}
	var resp messages.SessionInfoResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.SessionID != wantSession {
		t.Errorf("session_id: got %q, want %q", resp.SessionID, wantSession)
	}
	if resp.ProfileID != p.ID {
		t.Errorf("profile_id: got %q, want %q", resp.ProfileID, p.ID)
	}
}

func TestBridge_GetStoragePolicy(t *testing.T) {
	log := logging.Nop()
	aud := audit.New(log)
	p := identity.Windows11DesktopTemplate()
	p.Storage.EnableCookies = true
	p.Storage.EnableLocalStorage = false
	p.Storage.MaxStorageMB = 100
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "test-session", log, aud)
	br.Register(srv)
	result, err := srv.Call(context.Background(), messages.MethodStorageGetPolicy, nil)
	if err != nil {
		t.Fatalf("Call %s: %v", messages.MethodStorageGetPolicy, err)
	}
	var resp messages.StoragePolicyResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.EnableCookies {
		t.Error("expected enable_cookies=true")
	}
	if resp.EnableLocalStorage {
		t.Error("expected enable_local_storage=false")
	}
	if resp.MaxStorageMB != 100 {
		t.Errorf("max_storage_mb: got %d, want 100", resp.MaxStorageMB)
	}
}

func TestBridge_GetNetworkProfile(t *testing.T) {
	log := logging.Nop()
	aud := audit.New(log)
	p := identity.Windows11DesktopTemplate()
	p.Network.Timezone = "Europe/Amsterdam"
	p.Network.ProxyURL = "socks5://127.0.0.1:1080"
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "test-session", log, aud)
	br.Register(srv)
	result, err := srv.Call(context.Background(), messages.MethodNetworkGetProfile, nil)
	if err != nil {
		t.Fatalf("Call %s: %v", messages.MethodNetworkGetProfile, err)
	}
	var resp messages.NetworkProfileResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Timezone != "Europe/Amsterdam" {
		t.Errorf("timezone: got %q, want Europe/Amsterdam", resp.Timezone)
	}
	if resp.ProxyURL != "socks5://127.0.0.1:1080" {
		t.Errorf("proxy_url: got %q", resp.ProxyURL)
	}
}

func TestBridge_RendererEvent(t *testing.T) {
	_, srv := newTestBridge(t)
	ev := messages.RendererEvent{
		Type:      messages.EventRendererReady,
		SessionID: "test-session",
		Message:   "renderer started",
	}
	params, _ := json.Marshal(ev)
	result, err := srv.Call(context.Background(), messages.MethodRendererEvent, params)
	if err != nil {
		t.Fatalf("Call %s: %v", messages.MethodRendererEvent, err)
	}
	var resp map[string]bool
	if err := json.Unmarshal(result, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp["ok"] {
		t.Error("expected ok=true from renderer event handler")
	}
}
