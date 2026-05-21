# Security Model

## Threat Model

Ghost-Silicon treats the web as untrusted and tries to prevent websites from
learning real device hardware or tracking the user across sessions.

In embedded-WebView2 mode the WebView2 runtime itself is trusted (it is the
system Edge engine). The threat model focuses on **fingerprinting resistance**
and **data isolation**, not renderer exploit containment.

When an external renderer is configured (`engine.executable`), the renderer
process is additionally untrusted and isolated via Job Object + restricted
token (see `process-isolation.md`).

| Threat | Mitigation |
|---|---|
| Site reads real hardware values | Identity polyfill overrides all JS APIs |
| Site links sessions via fingerprint | Profile chosen fresh each launch via wizard |
| Site reads real GPU | WebGL `UNMASKED_VENDOR/RENDERER` overridden in polyfill |
| Site reads real canvas entropy | Deterministic noise injected per canvas seed |
| Site reads real audio entropy | Deterministic noise injected per audio seed |
| Site links sessions via cookies | Per-session `DataPath`; cleared between sessions |
| Cross-session data leakage | Isolated per-session directory trees |
| Renderer (external) reads real hardware | All values flow through the bridge |
| Renderer (external) writes outside session dir | Per-session ACLs + restricted token |
| Renderer (external) spawns arbitrary children | Job Object kill-on-close |
| Bridge pipe hijacking | DACL restricts pipe to current user only |
| Local API exposed to network | API binds only to `127.0.0.1` |
| Secret data leaked | DPAPI encryption; secrets never in env vars |

## Identity Isolation

Each launch the user picks an identity from the setup wizard. The polyfill
overrides:

- `navigator.userAgent`, `appVersion`, `vendor`, `platform`
- `navigator.hardwareConcurrency`, `deviceMemory`
- `navigator.languages`, `language`
- `screen.width/height/availWidth/availHeight/colorDepth/orientation`
- `window.devicePixelRatio`
- `Intl.DateTimeFormat` timezone
- WebGL `UNMASKED_VENDOR_WEBGL` / `UNMASKED_RENDERER_WEBGL`
- `HTMLCanvasElement` image data (noise seed)
- `AudioContext` channel data (noise seed)

The real hardware values are never exposed to page scripts.

## Session Isolation

Each session gets its own WebView2 `DataPath`:
```
%APPDATA%\ghost-silicon\sessions\<session-id>\
```
Cookies, localStorage, IndexedDB, cache, and service workers are isolated
per session. There is no cross-session storage sharing.

## Isolation Layers (external renderer)

```
┌─────────────────────────────────────────────────────┐
│  Windows Job Object                                  │
│  ┌─────────────────────────────────────────────────┐ │
│  │  Restricted Token (medium integrity by default) │ │
│  │  ┌─────────────────────────────────────────────┐│ │
│  │  │  Per-session directory (ACL: owner only)    ││ │
│  │  │  ┌────────────────────────────────────────┐ ││ │
│  │  │  │  Renderer process                      │ ││ │
│  │  │  └────────────────────────────────────────┘ ││ │
│  │  └─────────────────────────────────────────────┘│ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

## Named Pipe Security

The bridge pipe is secured with a DACL granting `GENERIC_ALL` only to the
current user's SID:

```
SDDL: D:(A;;GA;;;<current-user-SID>)
```

Other local users cannot connect to the bridge and inject spoofed responses.

## Secret Storage

Sensitive data is stored via Windows DPAPI. DPAPI binds ciphertext to the
current user account and machine — data cannot be decrypted on another
machine or by another user.

Files stored under `%APPDATA%\ghost-silicon\secrets\` with owner-only ACLs.

## Audit Trail

Every security-relevant event is written to an append-only NDJSON audit log:
- Profile loaded / saved / rotated / deleted
- Session started / stopped
- Renderer started / stopped / crashed (external mode)
- Permission granted / denied
- Network request allowed / blocked
- Config loaded

The audit log is separate from the application log and is never truncated.

## Safe Defaults

| Setting | Default | Rationale |
|---|---|---|
| `sandbox.enable_job_object` | `true` | Always protect process tree (external renderer) |
| `sandbox.enable_restricted_token` | `true` | Always reduce privileges (external renderer) |
| `sandbox.integrity_level` | `medium` | Standard Windows integrity |
| `api.enabled` | `false` | API off by default |
| `network.tls_skip_verify` | `false` | Never skip TLS verification |
| `storage.encrypt_at_rest` | `false` | Opt-in; requires key management |
| `ipc.enable_websocket` | `false` | Named pipe is default transport |

## What Ghost-Silicon Does NOT Do

- Does not bypass TLS certificate validation in production
- Does not implement TLS fingerprint evasion
- Does not modify system-wide registry settings
- Does not persist data outside `%APPDATA%\ghost-silicon\`
- Does not open any listening network socket by default
- Does not write to the Windows Event Log
- Does not escalate privileges
