# Ghost-Silicon Architecture

## Overview

Ghost-Silicon is a Go-based browser supervisor. It does not implement a browser
engine — it wraps a generic rendering engine and controls the environment that
engine runs in.

```
┌─────────────────────────────────────────────────────────┐
│                  Ghost-Silicon Supervisor                │
│                                                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────┐  │
│  │ Identity │  │ Network  │  │ Sandbox  │  │  IPC   │  │
│  │ Profiles │  │  Layer   │  │ Manager  │  │ Bridge │  │
│  └──────────┘  └──────────┘  └──────────┘  └────────┘  │
│                                                  │       │
└──────────────────────────────────────────────────┼───────┘
                                                   │ Named Pipe
                                          ┌────────┴───────┐
                                          │ Renderer Engine │
                                          │  (any Chromium  │
                                          │  compatible)    │
                                          └────────────────┘
```

## Core Components

### Supervisor (`internal/app/supervisor`)

The top-level orchestrator. Owns one session at a time and coordinates:
- Profile loading and rotation
- Engine lifecycle (start, monitor, restart, stop)
- IPC bridge wiring
- Lifecycle hook management

### Identity / Profile (`pkg/identity`)

Every value the renderer sees flows through a Profile. Fields:
- `hardware` — CPU cores, GPU vendor/renderer, RAM, platform, touch points
- `browser` — User-Agent, app version, vendor, languages, DoNotTrack
- `screen` — width, height, availWidth, availHeight, DPR, orientation
- `network` — timezone, proxy URL, DNS servers
- `noise` — canvas, audio, WebGL, font seeds
- `storage` — API enablement flags and quota caps
- `permissions` — per-permission states (deny / prompt / grant)

### IPC Bridge (`pkg/bridge`, `internal/ipc`)

The renderer connects to the supervisor over a Windows Named Pipe and calls
JSON-RPC methods. The bridge handlers look up values from the active Profile
and return them. The renderer never reads host hardware directly.

```
JavaScript API call
      ↓
Renderer hardware abstraction layer
      ↓
Named Pipe → JSON-RPC request
      ↓
Bridge handler (reads Profile)
      ↓
Profile-backed response
```

### Network Layer (`pkg/network`)

A custom `http.RoundTripper` wraps all renderer outbound traffic:
- Host ACL (allowlist / blocklist with wildcard support)
- Proxy routing (HTTP, HTTPS, SOCKS5)
- DNS override
- Header policy (strip internal headers, inject Accept-Language)
- TLS policy (minimum TLS 1.2)
- Connection pool management

### Sandbox (`internal/platform/windows`, `pkg/sandbox`)

Windows-first isolation using:
- **Job Objects** — groups all renderer child processes; kill-on-close prevents
  orphaned processes surviving the supervisor
- **Restricted tokens** — removes dangerous privileges; sets medium integrity
- **Per-session filesystem** — each session gets an isolated directory tree
  under `%APPDATA%\ghost-silicon\sessions\<session-id>\`
- **Firewall rules** — optional per-session Windows Firewall rules (netsh)

### Telemetry (`internal/telemetry`)

- Structured logging via `log/slog` (text or JSON format)
- In-process metrics (counters, histograms)
- Lightweight span tracing
- Append-only NDJSON audit log for security events

## Data Flow

```
Startup
  │
  ├─ Load config (YAML + env overrides)
  ├─ Run storage migrations
  ├─ Open profile store
  ├─ Load or generate active profile
  ├─ Create session ID
  ├─ Create session directory layout
  │
  ├─ Start IPC bridge (named pipe + JSON-RPC server)
  ├─ Launch renderer (Job Object + restricted token)
  │
  └─ Block until OS signal
       │
       └─ Graceful shutdown (LIFO hook order)
```

## Package Dependency Graph

```
cmd/ghost-silicon
  └── internal/app/bootstrap
        ├── internal/config/loader
        ├── internal/config/validation
        ├── internal/telemetry/logging
        ├── internal/telemetry/audit
        └── internal/app/supervisor
              ├── pkg/identity
              ├── pkg/bridge
              │     └── internal/ipc/jsonrpc
              ├── pkg/network
              ├── internal/engine/runtime
              │     └── internal/platform/windows/...
              └── internal/app/lifecycle
```

## Thread Safety

- `identity.FileStore` — all methods protected by `sync.RWMutex`
- `bridge.Bridge` — profile pointer swapped atomically via `UpdateProfile`
- `jsonrpc.Server` — handlers called concurrently; each handler is stateless
- `audit.Logger` — writes protected by `sync.Mutex`
- `metrics.Registry` — counters use `sync/atomic`; histogram protected by mutex