# Bridge Model

The bridge is the middleware between the supervisor's identity profile and
the renderer's hardware/navigator API requests.

## Concept

The renderer process never reads host hardware values directly. Every value
it sees flows through the bridge:

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

| Method                    | Returns                          |
|---------------------------|----------------------------------|
| `hardware.getCPUCores`    | `{ "cores": 8 }`                |
| `hardware.getRAM`         | `{ "device_memory_gb": 4 }`     |
| `hardware.getGPU`         | `{ "vendor": "...", "renderer": "..." }` |

### Navigator

| Method                    | Returns                          |
|---------------------------|----------------------------------|
| `navigator.getProfile`    | Full navigator object            |

### Screen

| Method                    | Returns                          |
|---------------------------|----------------------------------|
| `screen.getProfile`       | Full screen geometry object      |

### Noise Seeds

| Method                    | Returns                          |
|---------------------------|----------------------------------|
| `noise.getCanvasSeed`     | `{ "seed": 7493852614927364 }`   |
| `noise.getAudioSeed`      | `{ "seed": 3829473615728394 }`   |
| `noise.getWebGLSeed`      | `{ "seed": 5847362917483920 }`   |
| `noise.getFontSeed`       | `{ "seed": 1928374650183746 }`   |

### Storage and Permissions

| Method                       | Returns                       |
|------------------------------|-------------------------------|
| `storage.getPolicy`          | Storage API enablement flags  |
| `storage.getPaths`           | Per-session directory paths   |
| `permissions.getState`       | `{ "state": "deny" }`        |

### Session and Network

| Method                    | Returns                          |
|---------------------------|----------------------------------|
| `session.getInfo`         | `{ "session_id": "...", "profile_id": "..." }` |
| `network.getProfile`      | `{ "timezone": "...", ... }`     |

### Renderer Events (renderer → supervisor)

| Method                    | Payload                          |
|---------------------------|----------------------------------|
| `renderer.event`          | `RendererEvent` struct           |

## Wire Format

All messages use a 4-byte big-endian length prefix followed by a JSON-RPC 2.0
payload:

```
┌──────────────────────────┬───────────────────────────────────┐
│  4 bytes (uint32 BE)     │  N bytes (JSON-RPC 2.0 object)    │
│  payload length          │  { "jsonrpc": "2.0", ... }        │
└──────────────────────────┴───────────────────────────────────┘
```

Maximum frame size: 4 MB (enforced on both sides).

## Profile Hot-Swap

When the rotation policy triggers a profile change, `Bridge.UpdateProfile`
swaps the profile pointer atomically. All subsequent RPC calls use the new
profile immediately without restarting the renderer or reconnecting the pipe.

## Permission Enforcement

The `internal/ipc/permissions` package enforces method-level access control.
A renderer connection is only permitted to call the methods in the
`DefaultPolicy()` allowlist. Supervisor-internal methods (config reload,
session management) are only accessible via the local HTTP control API.

## Extending the Bridge

Register additional methods on the `jsonrpc.Server` before starting the pipe
listener:

```go
srv.Register("app.ping", jsonrpc.NoParams(func(ctx context.Context) (any, error) {
    return map[string]bool{"pong": true}, nil
}))
```

See `examples/named-pipe-bridge/handler.go` for a complete example.