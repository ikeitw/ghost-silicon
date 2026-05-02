// test/bridge/canvas_test.go
package bridge_test

import (
	"context"
	"encoding/json"
	"testing"

	"ghost-silicon/internal/ipc/messages"
	"ghost-silicon/pkg/identity"
)

func TestBridge_GetCanvasSeed_Zero(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Noise.CanvasSeed = 0
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodNoiseGetCanvasSeed, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var resp messages.NoiseSeedResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Seed != 0 {
		t.Errorf("expected seed=0, got %d", resp.Seed)
	}
}

func TestBridge_GetCanvasSeed_NonZero(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Noise.CanvasSeed = 7493852614927364
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodNoiseGetCanvasSeed, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var resp messages.NoiseSeedResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Seed != 7493852614927364 {
		t.Errorf("expected seed=7493852614927364, got %d", resp.Seed)
	}
}

func TestBridge_AllNoiseSeeds(t *testing.T) {
	p := identity.HardenedTemplate()
	srv := newBridgeServer(t, p)

	methods := []string{
		messages.MethodNoiseGetCanvasSeed,
		messages.MethodNoiseGetAudioSeed,
		messages.MethodNoiseGetWebGLSeed,
		messages.MethodNoiseGetFontSeed,
	}
	for _, m := range methods {
		raw, err := srv.Call(context.Background(), m, nil)
		if err != nil {
			t.Errorf("Call %s: %v", m, err)
			continue
		}
		var resp messages.NoiseSeedResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			t.Errorf("unmarshal %s: %v", m, err)
		}
	}
}

func TestBridge_GetPermissionState_Deny(t *testing.T) {
	p := identity.HardenedTemplate()
	srv := newBridgeServer(t, p)

	params, _ := json.Marshal(messages.PermissionQueryRequest{Permission: "camera"})
	raw, err := srv.Call(context.Background(), messages.MethodPermissionsGetState, params)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var resp messages.PermissionStateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.State != "deny" {
		t.Errorf("expected deny for camera on hardened profile, got %q", resp.State)
	}
}

func TestBridge_GetPermissionState_Unknown(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	srv := newBridgeServer(t, p)

	params, _ := json.Marshal(messages.PermissionQueryRequest{Permission: "unknown-api"})
	raw, err := srv.Call(context.Background(), messages.MethodPermissionsGetState, params)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var resp messages.PermissionStateResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.State != "deny" {
		t.Errorf("unknown permission should default to deny, got %q", resp.State)
	}
}
