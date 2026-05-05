// test/bridge/hardware_test.go
package bridge_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/ipc/messages"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/bridge"
	"ghost-silicon/pkg/identity"
)

func newBridgeServer(t *testing.T, p *identity.Profile) *jsonrpc.Server {
	t.Helper()
	log := logging.Nop()
	aud := audit.New(io.Discard)
	srv := jsonrpc.NewServer(log)
	br := bridge.New(p, "test-session", log, aud)
	br.Register(srv)
	return srv
}

func TestBridge_GetCPUCores(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.CPUCores = 12
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodHardwareGetCPUCores, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.HardwareCPUCoresResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Cores != 12 {
		t.Errorf("expected 12 cores, got %d", resp.Cores)
	}
}

func TestBridge_GetRAM(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.RAMMb = 8192 // 8 GB → deviceMemory = 4
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodHardwareGetRAM, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.HardwareRAMResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.DeviceMemoryGB != 4.0 {
		t.Errorf("expected 4.0 GB device memory, got %v", resp.DeviceMemoryGB)
	}
}

func TestBridge_GetGPU(t *testing.T) {
	p := identity.Windows11DesktopTemplate()
	p.Hardware.GPUVendor = "Google Inc. (AMD)"
	p.Hardware.GPURenderer = "ANGLE (AMD, AMD Radeon RX 6600 Direct3D11 vs_5_0 ps_5_0, D3D11)"
	srv := newBridgeServer(t, p)

	raw, err := srv.Call(context.Background(), messages.MethodHardwareGetGPU, nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	var resp messages.HardwareGPUResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Vendor != "Google Inc. (AMD)" {
		t.Errorf("gpu vendor: got %q, want %q", resp.Vendor, "Google Inc. (AMD)")
	}
}
