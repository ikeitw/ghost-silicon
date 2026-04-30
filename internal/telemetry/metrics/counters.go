// internal/telemetry/metrics/counters.go
// Package metrics — well-known counter names used across ghost-silicon.
// Import this file to get typed counter accessors on the global registry.
package metrics

// Well-known counter names.
const (
	CounterRendererStarts   = "renderer.starts_total"
	CounterRendererCrashes  = "renderer.crashes_total"
	CounterRendererRestarts = "renderer.restarts_total"
	CounterRPCRequests      = "ipc.rpc_requests_total"
	CounterRPCErrors        = "ipc.rpc_errors_total"
	CounterNetworkRequests  = "network.requests_total"
	CounterNetworkBlocked   = "network.blocked_total"
	CounterProfileLoads     = "identity.profile_loads_total"
	CounterProfileRotations = "identity.profile_rotations_total"
	CounterSessionsStarted  = "session.started_total"
	CounterSessionsStopped  = "session.stopped_total"
)

// RendererStarts returns the global renderer-starts counter.
func RendererStarts() *Counter { return Global.Counter(CounterRendererStarts) }

// RendererCrashes returns the global renderer-crashes counter.
func RendererCrashes() *Counter { return Global.Counter(CounterRendererCrashes) }

// RendererRestarts returns the global renderer-restarts counter.
func RendererRestarts() *Counter { return Global.Counter(CounterRendererRestarts) }

// RPCRequests returns the global IPC RPC request counter.
func RPCRequests() *Counter { return Global.Counter(CounterRPCRequests) }

// RPCErrors returns the global IPC RPC error counter.
func RPCErrors() *Counter { return Global.Counter(CounterRPCErrors) }

// NetworkRequests returns the global network request counter.
func NetworkRequests() *Counter { return Global.Counter(CounterNetworkRequests) }

// NetworkBlocked returns the global network blocked counter.
func NetworkBlocked() *Counter { return Global.Counter(CounterNetworkBlocked) }

// ProfileRotations returns the global profile rotation counter.
func ProfileRotations() *Counter { return Global.Counter(CounterProfileRotations) }
