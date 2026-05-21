# Windows Runtime

Ghost-Silicon is a Windows 11 browser. This document covers Windows-specific
runtime requirements, isolation primitives, and Win32 integration.

## Requirements

| Requirement | Minimum |
|---|---|
| OS | Windows 11 (build 22000+) |
| Architecture | amd64 |
| Go | 1.22+ |
| WebView2 Runtime | Microsoft Edge WebView2 (ships with Windows 11) |
| Privileges | Standard user (no admin needed for basic operation) |

## Frameless Window

The browser chrome (tab strip, address bar) is an HTML overlay. The OS title
bar is removed so it does not overlap.

In `pkg/browser/webview.go` (`NewWebViewPanel`):

```go
// Remove OS caption bar
curStyle := win.GetWindowLong(hwnd, win.GWL_STYLE)
win.SetWindowLong(hwnd, win.GWL_STYLE, curStyle &^ win.WS_CAPTION | win.WS_CLIPCHILDREN)
win.SetWindowPos(hwnd, 0, 0, 0, 0, 0, SWP_FRAMECHANGED|...)

// Restore DWM drop-shadow and Windows 11 rounded corners
dwmapi.DwmExtendFrameIntoClientArea(hwnd, &margins{0,0,1,0})
dwmapi.DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, DWMWCP_ROUND)
```

A custom WndProc (`subclassFrameless`) handles:
- `WM_NCCALCSIZE` — removes the non-client area so WebView2 fills the entire frame
- `WM_NCHITTEST` — returns `HTCLIENT` everywhere (drag and resize handled in JS)

## Job Objects (external renderer mode)

When `engine.executable` is set, the renderer process is placed in a Job
Object at creation time.

Key settings:
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — all child processes terminated when
  the supervisor exits; no orphaned renderers
- `JOB_OBJECT_LIMIT_JOB_MEMORY` — optional memory cap (bytes)
- `JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | HARD_CAP` — optional CPU rate cap

The renderer is launched with `CREATE_SUSPENDED`, assigned to the Job Object,
then resumed via `ResumeThread`. This prevents escaping the job between
creation and assignment.

## Restricted Process Tokens (external renderer mode)

When `sandbox.enable_restricted_token: true`:

1. `DuplicateTokenEx` — copy current process token
2. `AdjustTokenPrivileges` with `SE_PRIVILEGE_REMOVED` — strips dangerous privileges
3. `SetTokenInformation(TokenIntegrityLevel)` — sets Medium (or Low) integrity

Privileges removed by default:

| Privilege | Why |
|---|---|
| `SeDebugPrivilege` | Cannot attach debugger to other processes |
| `SeLoadDriverPrivilege` | Cannot load kernel drivers |
| `SeTcbPrivilege` | Cannot act as OS |
| `SeBackupPrivilege` | Cannot bypass file ACLs for reads |
| `SeRestorePrivilege` | Cannot bypass file ACLs for writes |
| `SeCreateTokenPrivilege` | Cannot forge tokens |
| `SeTakeOwnershipPrivilege` | Cannot seize object ownership |
| `SeAssignPrimaryTokenPrivilege` | Cannot swap process tokens |
| `SeImpersonatePrivilege` | Cannot impersonate other users |
| `SeCreateGlobalPrivilege` | Cannot create global kernel objects |

## Per-Session Filesystem

Created by `filesystem.CreateSessionLayout` for every run:

```
%APPDATA%\ghost-silicon\sessions\<session-id>\
├── cache\            HTTP cache (--disk-cache-dir for external renderers)
├── cookies\          Cookie storage
├── local_data\       localStorage / IndexedDB
├── downloads\        Default download location
├── extensions\       Extension data
├── logs\
│   └── renderer.log
└── session.json      Session metadata
```

In embedded WebView2 mode, `<session-id>\` is passed as `DataPath` to
go-webview2, providing automatic per-session isolation for cookies,
localStorage, and cache.

ACL applied by `filesystem.LockToCurrentUser`:
```
SDDL: O:<SID>G:<SID>D:(A;OICI;FA;;;<SID>)
```
Only the owning user has Full Control.

## Named Pipe Security

The bridge pipe is created with a DACL granting `GENERIC_ALL` only to the
current user's SID:

```
SDDL: D:(A;;GA;;;<current-user-SID>)
```

Pipe name: `\\.\pipe\ghost-silicon-bridge` (configurable via `ipc.pipe_name`).

## Windows Firewall (optional)

Per-session firewall rules via `netsh advfirewall`:

1. **Outbound allow** — permits all outbound traffic from the session
2. **Inbound block** — blocks all non-loopback inbound connections

Rules are tagged with the session ID and removed on session stop.

## Registry Access

Ghost-Silicon reads two registry paths at startup:

- `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings` —
  system proxy configuration (fallback when `network.proxy_url` is empty)
- `HKLM\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces` —
  system DNS servers (fallback when `network.dns_servers` is empty)

No registry values are written.

## DWM APIs Used

| API | Purpose |
|---|---|
| `DwmExtendFrameIntoClientArea` | 1-pixel top margin restores shadow |
| `DwmSetWindowAttribute(DWMWA_NCRENDERING_POLICY)` | Disable non-client rendering |
| `DwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE)` | Windows 11 rounded corners |

## AppContainer (Planned)

AppContainer-style isolation is planned. Enable with:

```yaml
sandbox:
  enable_app_container: true
```

Full `CreateAppContainerProfile` + capability SID integration is in the backlog.
