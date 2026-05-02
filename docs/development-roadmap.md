# Development Roadmap

## Phase 1 — Windows Supervisor (complete)

- [x] CLI entry point with config flag
- [x] YAML config loader with env overrides
- [x] Config validation
- [x] Profile model, validation, consistency checks
- [x] Profile store (atomic file writes)
- [x] Profile generator (randomised realistic values)
- [x] Built-in profile templates
- [x] Profile import / export
- [x] Profile rotation policies
- [x] Structured logging (log/slog)
- [x] Audit trail (NDJSON)
- [x] In-process metrics and tracing
- [x] Bootstrap and lifecycle manager
- [x] Graceful shutdown with signal handling

## Phase 2 — Windows Process Isolation (complete)

- [x] Windows Job Object (kill-on-close, memory/CPU limits)
- [x] Restricted process token (privilege removal)
- [x] Integrity level configuration (low / medium / high)
- [x] Per-session directory tree creation
- [x] Session directory ACL (owner-only)
- [x] Renderer process launcher (CREATE_SUSPENDED → assign job → resume)
- [x] stdout/stderr capture via anonymous pipes
- [x] Crash restart loop with exponential backoff
- [x] Windows Firewall rule management (netsh, optional)

## Phase 3 — IPC Bridge (complete)

- [x] Length-prefixed frame protocol
- [x] JSON-RPC 2.0 codec
- [x] JSON-RPC server with method dispatcher
- [x] Named pipe server and client
- [x] Bridge handlers for all profile subsystems
- [x] Renderer event notifications (renderer → supervisor)
- [x] Permission-level method access control
- [x] Profile hot-swap on rotation
- [x] Optional WebSocket transport

## Phase 4 — Network Layer (complete)

- [x] Custom http.RoundTripper
- [x] Custom TCP dialer with configurable timeouts
- [x] Custom DNS resolver with server override
- [x] Proxy support (HTTP, HTTPS, SOCKS5)
- [x] Host ACL (allowlist / blocklist with wildcard support)
- [x] Header policy (strip, inject, set-if-missing)
- [x] TLS policy (minimum version enforcement)
- [x] Connection pool configuration
- [x] Windows system proxy reader (registry)
- [x] Windows system DNS reader (registry)

## Phase 5 — Storage (complete)

- [x] Profile store (atomic JSON file writes)
- [x] Session store (metadata persistence)
- [x] Cache store with size reporting
- [x] Cookie store with quota checking
- [x] Permission store (per-origin decisions)
- [x] AES-256-GCM encrypted key-value store
- [x] Windows AppData path resolution
- [x] Storage schema migrations

## Phase 6 — Security Utilities (complete)

- [x] DPAPI secret store (Windows)
- [x] In-memory keyring fallback
- [x] Isolation probe (Job Object, token availability)
- [x] Safe defaults checker
- [x] Loopback address enforcement
- [x] Security audit event helpers

## Phase 7 — Developer Tooling (complete)

- [x] Profile inspector (validate, diff, export)
- [x] Sandbox checker (isolation prerequisites)
- [x] Network debugger (transport test with ACL)
- [x] Renderer mock (pipe client sending bridge requests)
- [x] Windows environment checker

## Phase 8 — Optional Local API (complete)

- [x] HTTP control API server (loopback only)
- [x] GET /health
- [x] GET/POST /profiles, GET/DELETE /profiles/{id}
- [x] GET /sessions, GET/POST /sessions/{id}/stop
- [x] GET /network/status
- [x] Logging middleware
- [x] Recovery middleware
- [x] Bearer token authentication

## Phase 9 — Linux Backend (stub complete, full impl pending)

- [x] Linux process launcher (plain exec.Cmd)
- [x] Namespace stubs (UTS, net, mnt, user, PID)
- [x] cgroup v2 stubs (memory, CPU, pids)
- [x] seccomp allowlist / blocklist defined
- [x] Overlay and bind mount stubs
- [ ] Full namespace isolation via clone(2)
- [ ] cgroup v2 write and enforcement
- [ ] seccomp-BPF filter installation
- [ ] Overlay filesystem for read-only base image

## Phase 10 — macOS Backend (stub complete, full impl pending)

- [x] macOS process launcher (plain exec.Cmd)
- [x] Seatbelt profile stub
- [ ] sandbox-exec integration with deny-default profile
- [ ] App Sandbox entitlement configuration

## Phase 11 — AppContainer (planned)

- [ ] `CreateAppContainerProfile` via COM API
- [ ] `DeleteAppContainerProfile`
- [ ] Capability SID construction
- [ ] `CreateProcess` with `PROC_THREAD_ATTRIBUTE_SECURITY_CAPABILITIES`

## Phase 12 — Extended Profile Features (planned)

- [ ] Profile templates stored in database
- [ ] Profile schema validation via JSON Schema
- [ ] Bulk profile generation CLI
- [ ] Profile diff and merge tooling
- [ ] Profile signing (Ed25519)

## Phase 13 — Observability (planned)

- [ ] Prometheus /metrics endpoint
- [ ] OpenTelemetry trace export
- [ ] Renderer crash dump capture
- [ ] Structured audit log viewer