// internal/ipc/permissions/policy.go
package permissions

import "ghost-silicon/internal/ipc/messages"

// Policy lists the RPC methods that renderer processes are allowed to call.
// Everything not on this list is denied before the handler is even invoked.
type Policy struct {
	allowed map[string]struct{}
}

// DefaultPolicy returns the standard set of methods a renderer may call.
// Supervisor-internal methods (config reload, session management) are not
// included — those are only accessible via the local HTTP control API.
func DefaultPolicy() *Policy {
	allowed := []string{
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
		messages.MethodStorageGetPaths,
		messages.MethodPermissionsGetState,
		messages.MethodSessionGetInfo,
		messages.MethodNetworkGetProfile,
		messages.MethodRendererEvent, // renderer → supervisor notification
	}
	p := &Policy{allowed: make(map[string]struct{}, len(allowed))}
	for _, m := range allowed {
		p.allowed[m] = struct{}{}
	}
	return p
}

// IsAllowed reports whether method is in the allowed set.
func (p *Policy) IsAllowed(method string) bool {
	_, ok := p.allowed[method]
	return ok
}

// Add adds a method to the allowed set at runtime.
func (p *Policy) Add(method string) { p.allowed[method] = struct{}{} }

// Remove removes a method from the allowed set.
func (p *Policy) Remove(method string) { delete(p.allowed, method) }
