// pkg/bridge/bridge.go
// Package bridge is the middle layer between the supervisor's profile store
// and the renderer's hardware/navigator API requests.  Every value the
// renderer sees flows through here — nothing reads host hardware directly.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"

	"ghost-silicon/internal/ipc/jsonrpc"
	"ghost-silicon/internal/ipc/messages"
	"ghost-silicon/internal/telemetry/audit"
	"ghost-silicon/internal/telemetry/logging"
	"ghost-silicon/pkg/identity"
)

// Bridge provides profile-backed responses to all renderer IPC requests.
// One Bridge is created per active session.
type Bridge struct {
	profile   *identity.Profile
	sessionID string
	log       *logging.Logger
	auditor   *audit.Logger
}

// New creates a Bridge backed by the given profile and session.
func New(
	profile *identity.Profile,
	sessionID string,
	log *logging.Logger,
	auditor *audit.Logger,
) *Bridge {
	return &Bridge{
		profile:   profile,
		sessionID: sessionID,
		log:       log.WithComponent("bridge"),
		auditor:   auditor,
	}
}

// Register mounts all bridge handlers onto the JSON-RPC server.
func (b *Bridge) Register(srv *jsonrpc.Server) {
	srv.Register(messages.MethodHardwareGetCPUCores, jsonrpc.NoParams(b.handleGetCPUCores))
	srv.Register(messages.MethodHardwareGetRAM, jsonrpc.NoParams(b.handleGetRAM))
	srv.Register(messages.MethodHardwareGetGPU, jsonrpc.NoParams(b.handleGetGPU))
	srv.Register(messages.MethodNavigatorGetProfile, jsonrpc.NoParams(b.handleGetNavigator))
	srv.Register(messages.MethodScreenGetProfile, jsonrpc.NoParams(b.handleGetScreen))
	srv.Register(messages.MethodNoiseGetCanvasSeed, jsonrpc.NoParams(b.handleGetCanvasSeed))
	srv.Register(messages.MethodNoiseGetAudioSeed, jsonrpc.NoParams(b.handleGetAudioSeed))
	srv.Register(messages.MethodNoiseGetWebGLSeed, jsonrpc.NoParams(b.handleGetWebGLSeed))
	srv.Register(messages.MethodNoiseGetFontSeed, jsonrpc.NoParams(b.handleGetFontSeed))
	srv.Register(messages.MethodStorageGetPolicy, jsonrpc.NoParams(b.handleGetStoragePolicy))
	srv.Register(messages.MethodPermissionsGetState, jsonrpc.WithParams[messages.PermissionQueryRequest](b.handleGetPermissionState))
	srv.Register(messages.MethodSessionGetInfo, jsonrpc.NoParams(b.handleGetSessionInfo))
	srv.Register(messages.MethodNetworkGetProfile, jsonrpc.NoParams(b.handleGetNetworkProfile))
	srv.Register(messages.MethodRendererEvent, jsonrpc.WithParams[messages.RendererEvent](b.handleRendererEvent))
}

// UpdateProfile hot-swaps the profile backing this bridge (used on rotation).
func (b *Bridge) UpdateProfile(p *identity.Profile) {
	b.profile = p
	b.log.Info("bridge profile updated", logging.FieldProfileID, p.ID)
}

// ── hardware handlers ────────────────────────────────────────────────────────

func (b *Bridge) handleGetCPUCores(_ context.Context) (any, error) {
	b.logRequest(messages.MethodHardwareGetCPUCores)
	return &messages.HardwareCPUCoresResponse{
		Cores: b.profile.Hardware.CPUCores,
	}, nil
}

func (b *Bridge) handleGetRAM(_ context.Context) (any, error) {
	b.logRequest(messages.MethodHardwareGetRAM)
	return &messages.HardwareRAMResponse{
		DeviceMemoryGB: b.profile.Hardware.DeviceMemoryGB(),
	}, nil
}

func (b *Bridge) handleGetGPU(_ context.Context) (any, error) {
	b.logRequest(messages.MethodHardwareGetGPU)
	return &messages.HardwareGPUResponse{
		Vendor:   b.profile.Hardware.GPUVendor,
		Renderer: b.profile.Hardware.GPURenderer,
	}, nil
}

// ── navigator handler ────────────────────────────────────────────────────────

func (b *Bridge) handleGetNavigator(_ context.Context) (any, error) {
	b.logRequest(messages.MethodNavigatorGetProfile)
	br := &b.profile.Browser
	return &messages.NavigatorProfileResponse{
		UserAgent:      br.UserAgent,
		AppVersion:     br.AppVersion,
		Vendor:         br.Vendor,
		VendorSub:      br.VendorSub,
		Product:        br.Product,
		ProductSub:     br.ProductSub,
		Languages:      br.Languages,
		DoNotTrack:     br.DoNotTrack,
		CookieEnabled:  br.CookieEnabled,
		MaxTouchPoints: b.profile.Hardware.MaxTouchPoints,
		Platform:       b.profile.Hardware.Platform,
	}, nil
}

// ── screen handler ───────────────────────────────────────────────────────────

func (b *Bridge) handleGetScreen(_ context.Context) (any, error) {
	b.logRequest(messages.MethodScreenGetProfile)
	s := &b.profile.Screen
	return &messages.ScreenProfileResponse{
		Width:            s.Width,
		Height:           s.Height,
		AvailWidth:       s.AvailWidth,
		AvailHeight:      s.AvailHeight,
		ColorDepth:       s.ColorDepth,
		PixelDepth:       s.PixelDepth,
		DevicePixelRatio: s.DevicePixelRatio,
		Orientation:      string(s.Orientation),
	}, nil
}

// ── noise handlers ───────────────────────────────────────────────────────────

func (b *Bridge) handleGetCanvasSeed(_ context.Context) (any, error) {
	return &messages.NoiseSeedResponse{Seed: b.profile.Noise.CanvasSeed}, nil
}
func (b *Bridge) handleGetAudioSeed(_ context.Context) (any, error) {
	return &messages.NoiseSeedResponse{Seed: b.profile.Noise.AudioSeed}, nil
}
func (b *Bridge) handleGetWebGLSeed(_ context.Context) (any, error) {
	return &messages.NoiseSeedResponse{Seed: b.profile.Noise.WebGLSeed}, nil
}
func (b *Bridge) handleGetFontSeed(_ context.Context) (any, error) {
	return &messages.NoiseSeedResponse{Seed: b.profile.Noise.FontSeed}, nil
}

// ── storage handler ──────────────────────────────────────────────────────────

func (b *Bridge) handleGetStoragePolicy(_ context.Context) (any, error) {
	b.logRequest(messages.MethodStorageGetPolicy)
	sp := &b.profile.Storage
	return &messages.StoragePolicyResponse{
		EnableCookies:        sp.EnableCookies,
		EnableLocalStorage:   sp.EnableLocalStorage,
		EnableSessionStorage: sp.EnableSessionStorage,
		EnableIndexedDB:      sp.EnableIndexedDB,
		EnableCacheStorage:   sp.EnableCacheStorage,
		EnableServiceWorker:  sp.EnableServiceWorker,
		MaxCookieJarMB:       sp.MaxCookieJarMB,
		MaxStorageMB:         sp.MaxStorageMB,
	}, nil
}

// ── permissions handler ──────────────────────────────────────────────────────

func (b *Bridge) handleGetPermissionState(_ context.Context, req messages.PermissionQueryRequest) (any, error) {
	b.logRequest(messages.MethodPermissionsGetState)
	state := b.permissionState(req.Permission)
	b.auditor.Log(audit.Event{
		Type:      audit.EventPermissionGrant,
		SessionID: b.sessionID,
		ProfileID: b.profile.ID,
		Actor:     "bridge",
		Target:    req.Permission,
		Outcome:   state,
	})
	return &messages.PermissionStateResponse{State: state}, nil
}

func (b *Bridge) permissionState(name string) string {
	p := &b.profile.Permissions
	switch name {
	case "geolocation":
		return string(p.Geolocation)
	case "notifications":
		return string(p.Notifications)
	case "microphone":
		return string(p.Microphone)
	case "camera":
		return string(p.Camera)
	case "clipboard":
		return string(p.Clipboard)
	case "full_screen":
		return string(p.FullScreen)
	case "payment_handler":
		return string(p.PaymentHandler)
	case "midi":
		return string(p.MIDI)
	case "usb":
		return string(p.USB)
	case "bluetooth":
		return string(p.Bluetooth)
	default:
		return string(identity.PermissionDeny) // deny unknown permissions
	}
}

// ── session / network handlers ───────────────────────────────────────────────

func (b *Bridge) handleGetSessionInfo(_ context.Context) (any, error) {
	return &messages.SessionInfoResponse{
		SessionID: b.sessionID,
		ProfileID: b.profile.ID,
	}, nil
}

func (b *Bridge) handleGetNetworkProfile(_ context.Context) (any, error) {
	n := &b.profile.Network
	return &messages.NetworkProfileResponse{
		Timezone:   n.Timezone,
		ProxyURL:   n.ProxyURL,
		DNSServers: n.DNSServers,
	}, nil
}

// ── renderer event handler ───────────────────────────────────────────────────

func (b *Bridge) handleRendererEvent(_ context.Context, ev messages.RendererEvent) (any, error) {
	b.log.Info("renderer event",
		"event_type", string(ev.Type),
		logging.FieldSessionID, ev.SessionID,
		logging.FieldURL, ev.URL,
	)
	b.auditor.Log(audit.Event{
		Type:      audit.EventType(ev.Type),
		SessionID: b.sessionID,
		ProfileID: b.profile.ID,
		Actor:     "renderer",
		Target:    ev.URL,
		Outcome:   "ok",
		Message:   ev.Message,
	})
	return map[string]bool{"ok": true}, nil
}

// logRequest emits a structured debug log for an incoming bridge request.
func (b *Bridge) logRequest(method string) {
	b.log.Debug("bridge request",
		logging.FieldRPCMethod, method,
		logging.FieldSessionID, b.sessionID,
		logging.FieldProfileID, b.profile.ID,
	)
}

// EncodeResult is a helper that marshals v and returns an error with context.
func EncodeResult(method string, v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("bridge: encode result for %s: %w", method, err)
	}
	return data, nil
}
