# Bridge Model

## Role in Embedded Mode

In the default embedded-WebView2 mode the bridge is not on the hot path.
Profile values are injected directly into the page via
`AddScriptToExecuteOnDocumentCreated` — the page never makes a round-trip
over the named pipe to get `navigator.userAgent`.

The bridge is still started and the named pipe is open, but its primary
purpose is to serve **external renderer adapters** (see `renderer-adapter.md`)
when `engine.executable` is set in config.

## Concept

The external renderer process never reads host hardware values directly.
Every value it sees flows through the bridge:

```
JavaScript API call (e.g. navigator.hardwareConcurrency)
        ↓
Renderer abstraction layer (injected script)
        ↓
Named Pipe → JSON-RPC request ("hardware.getCPUCores")
        ↓
Bridge handler (pkg/bridge/bridge.go)
        ↓
Active Profile lookup
        ↓
JSON-RPC response → { "cores": 8 }
        ↓
JavaScript receives controlled value
```

## RPC Methods

All methods use JSON-RPC 2.0 over a length-prefixed named pipe connection.

### Hardware

| Method | Returns |
|---|---|
| `hardware.getCPUCores` | `{ "cores": 8 }` |
| `hardware.getRAM` | `{ "device_memory_gb": 4 }` |
| `hardware.getGPU` | `{ "vendor": "...", "renderer": "..." }` |

### Navigator

| Method | Returns |
|---|---|
| `navigator.getProfile` | Full navigator object |

### Screen

| Method | Returns |
|---|---|
| `screen.getProfile` | Full screen geometry object |

### Noise Seeds

| Method | Returns |
|---|---|
| `noise.getCanvasSeed` | `{ "seed": 7493852614927364 }` |
| `noise.getAudioSeed` | `{ "seed": 3829473615728394 }` |
| `noise.getWebGLSeed` | `{ "seed": 5847362917483920 }` |
| `noise.getFontSeed` | `{ "seed": 1928374650183746 }` |

### Storage and Permissions

| Method | Returns |
|---|---|
| `storage.getPolicy` | Storage API enablement flags |
| `storage.getPaths` | Per-session directory paths |
| `permissions.getState` | `{ "state": "deny" }` |

### Session and Network

| Method | Returns |
|---|---|
| `session.getInfo` | `{ "session_id": "...", "profile_id": "..." }` |
| `network.getProfile` | `{ "timezone": "...", ... }` |

### Renderer Events (renderer → supervisor)

| Method | Payload |
|---|---|
| `renderer.event` | `RendererEvent` struct |

## Wire Format

4-byte big-endian length prefix followed by a JSON-RPC 2.0 payload:

```
┌──────────────────────────┬───────────────────────────────────┐
│  4 bytes (uint32 BE)     │  N bytes (JSON-RPC 2.0 object)    │
│  payload length          │  { "jsonrpc": "2.0", ... }        │
└──────────────────────────┴───────────────────────────────────┘
```

Maximum frame size: 4 MB (enforced on both sides).

## Profile Hot-Swap

`Bridge.UpdateProfile` swaps the profile pointer atomically. All subsequent
RPC calls use the new profile immediately without restarting or reconnecting.

## Permission Enforcement

`internal/ipc/permissions` enforces method-level access control. A renderer
connection may only call methods in the `DefaultPolicy()` allowlist.
Supervisor-internal methods are only accessible via the local HTTP control API.

## Extending the Bridge

Register additional methods before starting the pipe listener:

```go
srv.Register("app.ping", jsonrpc.NoParams(func(ctx context.Context) (any, error) {
    return map[string]bool{"pong": true}, nil
}))
```
