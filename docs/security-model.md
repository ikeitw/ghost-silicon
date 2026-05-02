# Security Model

## Threat Model

Ghost-Silicon assumes the renderer process is **untrusted**. The supervisor
acts as a reference monitor between the renderer and the host system.

Threats considered:

| Threat                              | Mitigation                                  |
|-------------------------------------|---------------------------------------------|
| Renderer reads real hardware values | All values flow through the bridge          |
| Renderer writes outside session dir | Per-session ACLs + restricted token         |
| Renderer spawns arbitrary children  | Job Object kill-on-close                    |
| Renderer survives supervisor exit   | Job Object kill-on-close                    |
| Renderer abuses kernel privileges   | Privilege removal from process token        |
| Cross-session data leakage          | Isolated per-session directory trees        |
| Bridge pipe hijacking               | DACL restricts pipe to current user only    |
| Local API exposed to network        | API binds only to 127.0.0.1                 |
| Secret data leaked to renderer      | DPAPI encryption, secrets never in env vars |

## Isolation Layers

```
┌─────────────────────────────────────────────────────┐
│  Windows Job Object                                  │
│  ┌─────────────────────────────────────────────────┐ │
│  │  Restricted Token (medium integrity)            │ │
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

The bridge pipe is created with a security descriptor that grants
`GENERIC_ALL` only to the current user's SID:

```
SDDL: D:(A;;GA;;;<current-user-SID>)
```

Other users on the same machine cannot connect to the bridge and inject
spoofed profile responses.

## Secret Storage

Sensitive data (encryption keys, saved tokens) is stored using the Windows
Data Protection API (DPAPI). DPAPI binds the ciphertext to the current user
account and machine — data cannot be decrypted on a different machine or by
a different user account.

Secret files are stored under `%APPDATA%\ghost-silicon\secrets\` with
permissions restricted to the owner (mode 0600 equivalent via ACLs).

## Audit Trail

Every security-relevant event is written to an append-only NDJSON audit log:

- Profile loaded / saved / rotated / deleted
- Session started / stopped
- Renderer started / stopped / crashed
- Permission granted / denied
- Network request allowed / blocked
- Sandbox violations
- Config loaded

The audit log is separate from the structured application log and is never
truncated — only rotated externally.

## Safe Defaults

Ghost-Silicon ships with conservative defaults:

| Setting                      | Default         | Rationale                         |
|------------------------------|-----------------|-----------------------------------|
| `sandbox.enable_job_object`  | `true`          | Always protect process tree       |
| `sandbox.enable_restricted_token` | `true`    | Always reduce privileges          |
| `sandbox.integrity_level`    | `medium`        | Standard Windows integrity        |
| `api.enabled`                | `false`         | API off by default                |
| `network.tls_skip_verify`    | `false`         | Never skip TLS verification       |
| `storage.encrypt_at_rest`    | `false`         | Opt-in, requires key management   |
| `ipc.enable_websocket`       | `false`         | Named pipe is default transport   |

## What Ghost-Silicon Does NOT Do

- Does not bypass TLS certificate validation in production
- Does not implement TLS fingerprint evasion
- Does not modify system-wide registry settings
- Does not persist data outside `%APPDATA%\ghost-silicon\`
- Does not open any listening network socket by default
- Does not write to the Windows Event Log
- Does not escalate privileges