# Ghost-Silicon

A Go-based browser supervisor for Windows 11. Ghost-Silicon wraps a generic
rendering engine with profile management, Windows process isolation, a local
IPC bridge, network policy, and per-session storage isolation.

## What it does

- **Profile management** — JSON identity profiles control hardware, navigator,
  screen, noise, storage, and permission values exposed to the renderer.
- **Windows process isolation** — Job Objects, restricted tokens, and
  per-session filesystem directories isolate each renderer session.
- **IPC bridge** — A named pipe JSON-RPC server answers renderer requests for
  profile-backed values (CPU cores, GPU, User-Agent, canvas seeds, etc.).
- **Network policy** — A custom `http.RoundTripper` enforces host ACLs, proxy
  routing, DNS policy, and header sanitisation.
- **Session storage** — Each session gets its own directory tree; cookies,
  cache, and local data never leak between sessions.

## Project layout

```
cmd/ghost-silicon/      Main entry point
internal/               Internal packages (not importable externally)
  app/                  Bootstrap, lifecycle, supervisor, shutdown
  config/               YAML loader, schema, validation, defaults
  engine/               Renderer adapter, launcher, health monitoring
  ipc/                  Named pipe server, WebSocket, JSON-RPC, messages
  platform/windows/     Job Objects, tokens, firewall, filesystem, registry
  policy/               Network, storage, JavaScript, permissions, sandbox
  telemetry/            Logging, metrics, tracing, audit
pkg/                    Public packages
  identity/             Profile model, validation, store, generator, rotation
  bridge/               IPC bridge handlers — profile → renderer values
  network/              Custom transport, dialer, resolver, proxy, TLS
  renderer/             Adapter interface, session, events, errors
  sandbox/              Cross-platform sandbox interface + Windows backend
  storage/              Profile store, session store, cache, cookies
  security/             DPAPI, keyring, isolation checks, safe defaults
  api/                  Local HTTP control API (optional)
  version/              Build version info
profiles/               Built-in profile JSON files
configs/                YAML configuration files
tools/                  Developer tooling
  profile-inspector/    Read and validate profile files
  sandbox-checker/      Check Windows isolation prerequisites
  network-debugger/     Test the network transport layer
  renderer-mock/        Simulate a renderer connecting over the pipe
  windows-env-checker/  Verify the Windows 11 environment
```

## Requirements

- **Windows 11** (build 22000+) for full isolation support
- **Go 1.22** or later
- A Chromium-compatible renderer binary (configure in `configs/ghost-silicon.yaml`)

## Quick start

### 1. Configure

Copy and edit the main config file:

```yaml
# configs/ghost-silicon.yaml
engine:
  executable: "C:\\path\\to\\your\\renderer.exe"
```

### 2. Build

```powershell
.\scripts\build.ps1
```

Or with `make`:

```bash
make build-windows
```

### 3. Run

```powershell
.\bin\ghost-silicon.exe -config configs\ghost-silicon.yaml
```

Development mode (verbose logging, no binary needed):

```powershell
.\scripts\dev.ps1
```

## Profiles

Profiles are JSON files in the `profiles/` directory. Built-in templates:

| Template              | Description                              |
|-----------------------|------------------------------------------|
| `windows11-desktop`   | 8-core, RTX 3060, 1920×1080             |
| `windows11-laptop`    | 4-core, Intel UHD, 1366×768             |
| `mobile-like`         | Touch-enabled, 390×844, 3× DPI          |
| `hardened`            | Privacy-maximised, all storage disabled  |

Generate a new randomised profile:

```go
gen := identity.NewGenerator(0)
p   := gen.Generate(nil)
```

## Testing

```powershell
.\scripts\test.ps1
```

Or:

```bash
make test
```

Windows-specific tests (Job Objects, tokens, firewall):

```powershell
go test .\test\windows\... -v
```

## Developer tools

```powershell
# Inspect a profile
.\bin\profile-inspector.exe profiles\default.profile.json --validate --consistency

# Check Windows isolation prerequisites
.\bin\sandbox-checker.exe

# Test the network transport
.\bin\network-debugger.exe https://example.com

# Check Windows 11 environment
.\bin\windows-env-checker.exe
```

## Configuration reference

See `configs/ghost-silicon.yaml` for all available options with comments.
Key sections:

- `engine` — renderer binary path and restart policy
- `identity` — profile directory, auto-generation, rotation policy
- `network` — proxy, DNS, timeouts, host ACL rules
- `sandbox` — Job Object, token, integrity level, memory/CPU caps
- `ipc` — named pipe name, optional WebSocket endpoint
- `telemetry` — log level, format, audit file
- `api` — optional local HTTP control API (disabled by default)

## Architecture

```
Ghost-Silicon Supervisor
│
├── Loads Profile (from profiles/ or auto-generates)
├── Creates Session (random ID, isolated directory tree)
│
├── Starts IPC Bridge (Named Pipe → JSON-RPC server)
│   └── Answers renderer requests with profile-backed values
│
├── Launches Renderer Process
│   ├── Windows Job Object (kill-on-close, memory/CPU limits)
│   ├── Restricted token (medium integrity, stripped privileges)
│   └── Per-session user-data-dir
│
└── Network Layer (all renderer HTTP traffic)
    ├── Custom RoundTripper with host ACL
    ├── Proxy support (HTTP, HTTPS, SOCKS5)
    └── Header policy (strip internal headers, inject Accept-Language)
```

## License

See [LICENSE](LICENSE).# antidetect_browser