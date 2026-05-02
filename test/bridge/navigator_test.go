// test/bridge/navigator_test.go
package bridge_test

import (
	"context"
	"encoding/json"
	"testing"

	"ghost-silicon/internal/ipc/messages"
	"ghost-silicon/pkg/identity"
)

func TestBridge_GetNavigator(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Browser.Languages = []string{"nl-NL", "nl", "en"}
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodNavigatorGetProfile, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.NavigatorProfileResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.UserAgent != p.Browser.UserAgent {
		t.Errorf("user_agent: got %q, want %q", resp.UserAgent, p.Browser.UserAgent)
	}
	if len(resp.Languages) != 3 {
		t.Errorf("languages: expected 3, got %d", len(resp.Languages))
	}
	if resp.Languages[0] != "nl-NL" {
		t.Errorf("first language: got %q, want %q", resp.Languages[0], "nl-NL")
	}
}

func TestBridge_GetNavigator_Platform(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.Platform = "Win32"
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodNavigatorGetProfile, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.NavigatorProfileResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Platform != "Win32" {
		t.Errorf("platform: got %q, want Win32", resp.Platform)
	}
}

func TestBridge_GetNavigator_MaxTouchPoints(t *testing.T) {
	p := identity.MobileLikeTemplate()
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodNavigatorGetProfile, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.NavigatorProfileResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.MaxTouchPoints != 5 {
		t.Errorf("max_touch_points: got %d, want 5", resp.MaxTouchPoints)
	}
}
