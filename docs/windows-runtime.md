# Windows Runtime

Ghost-Silicon is designed for Windows 11. This document describes the
Windows-specific isolation mechanisms it uses.

## Requirements

| Requirement        | Minimum                        |
|--------------------|--------------------------------|
| OS                 | Windows 11 (build 22000+)      |
| Architecture       | amd64                          |
| Go                 | 1.22+                          |
| Privileges         | Standard user (no admin needed for basic operation) |

## Job Objects

Every renderer process is placed inside a Windows Job Object on startup.

Key settings applied:
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — all child processes are terminated
  when the supervisor exits, preventing orphaned renderer processes.
- Optional memory limit via `JOB_OBJECT_LIMIT_JOB_MEMORY`.
- Optional CPU rate cap via `JobObjectCpuRateControlInformation`.

The Job Object is created with `CreateJobObject` and the renderer is assigned
immediately after `CreateProcess` (while the process is suspended via
`CREATE_SUSPENDED`) so the renderer cannot escape the job before execution begins.

## Restricted Process Tokens

When `sandbox.enable_restricted_token: true` (the default), the renderer is
launched via `CreateProcessAsUser` with a token that has been:

1. Duplicated from the current process with `DuplicateTokenEx`
2. Had dangerous privileges removed via `AdjustTokenPrivileges` with
   `SE_PRIVILEGE_REMOVED`
3. Had its mandatory integrity level lowered to Medium (or Low if configured)
   via `SetTokenInformation` with `TokenIntegrityLevel`

Privileges removed by default:
- `SeDebugPrivilege`
- `SeLoadDriverPrivilege`
- `SeTcbPrivilege`
- `SeBackupPrivilege` / `SeRestorePrivilege`
- `SeCreateTokenPrivilege`
- `SeTakeOwnershipPrivilege`
- `SeAssignPrimaryTokenPrivilege`
- `SeImpersonatePrivilege`
- `SeCreateGlobalPrivilege`

## Per-Session Filesystem

Each session gets its own isolated directory tree:

```
%APPDATA%\ghost-silicon\sessions\<session-id>\
  cache\          HTTP cache (--disk-cache-dir)
  cookies\        Cookie storage
  local_data\     localStorage / IndexedDB
  downloads\      Default download location
  extensions\     Extension data
  logs\           Renderer log files
  session.json    Session metadata
```

The session root is passed to the renderer via `--user-data-dir`. This means
cookies, localStorage, and cached data are completely isolated between sessions.

## Windows Firewall

Optional per-session firewall rules can be applied via `netsh advfirewall`.
These rules tag outbound traffic for the session and block unexpected inbound
connections.

Configure in `configs/sandbox-policy.yaml`.

## Named Pipe Security

The bridge named pipe is secured with a DACL that grants access only to the
current Windows user (owner). Other users on the same machine cannot connect
to the bridge pipe.

The pipe name format: `\\.\pipe\ghost-silicon-bridge`

Customise in `configs/ghost-silicon.yaml` under `ipc.pipe_name`.

## AppContainer (Experimental)

AppContainer-style isolation is stubbed in Phase 1. Enable it with:

```yaml
sandbox:
  enable_app_container: true
```

Full `CreateAppContainerProfile` integration is planned for Phase 2.

## Registry Access

The supervisor reads two registry paths at startup:

- `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings` —
  reads the system proxy configuration so it can default to the user's
  configured proxy when no `network.proxy_url` is set.

- `HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces` —
  reads system DNS server addresses as a fallback when no `network.dns_servers`
  list is configured.

No registry values are written by ghost-silicon.