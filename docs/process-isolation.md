# Process Isolation

> This document covers isolation for the **external renderer mode**
> (`engine.executable` set in config). In the default embedded-WebView2 mode,
> WebView2 manages its own process isolation internally via Chromium's
> multi-process architecture.

## Isolation Stack

```
┌────────────────────────────────────────────────┐
│  Windows Job Object                            │
│  • Kill-on-close (no orphaned processes)       │
│  • Optional memory cap                         │
│  • Optional CPU rate cap                       │
│                                                │
│  ┌──────────────────────────────────────────┐  │
│  │  Restricted Process Token                │  │
│  │  • Dangerous privileges removed          │  │
│  │  • Mandatory integrity: Medium (default) │  │
│  │                                          │  │
│  │  ┌────────────────────────────────────┐  │  │
│  │  │  Per-session Filesystem            │  │  │
│  │  │  • Isolated user-data-dir          │  │  │
│  │  │  • Owner-only ACLs                 │  │  │
│  │  │  • Separate cache, cookies, logs   │  │  │
│  │  │                                    │  │  │
│  │  │  Renderer Process (PID: N)         │  │  │
│  │  └────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────┘  │
└────────────────────────────────────────────────┘
```

## Job Object

Created with `CreateJobObject` in `internal/platform/windows/jobobject`.

Flags applied:
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — kills all processes in the job
  when the last handle is closed (supervisor exit)
- `JOB_OBJECT_LIMIT_JOB_MEMORY` — optional memory cap in bytes
- `JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | HARD_CAP` — optional CPU rate cap

The renderer is launched with `CREATE_SUSPENDED`, assigned to the Job Object,
then resumed via `ResumeThread`. This guarantees the process cannot escape
the job between creation and assignment.

Configure in `configs/ghost-silicon.yaml`:

```yaml
sandbox:
  enable_job_object: true
  memory_limit_mb: 0     # 0 = unlimited
  cpu_rate_percent: 0    # 0 = unlimited
```

## Token Restriction

Created with `DuplicateTokenEx` + `AdjustTokenPrivileges` + `SetTokenInformation`.

Privileges removed (`SE_PRIVILEGE_REMOVED`):

| Privilege | Why removed |
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

Integrity level set to Medium by default. Low is available for maximum
isolation (some WebView2 features may break at Low).

Configure:

```yaml
sandbox:
  enable_restricted_token: true
  integrity_level: medium   # low | medium | high
```

## Per-Session Filesystem

Created by `filesystem.CreateSessionLayout`:

```
%APPDATA%\ghost-silicon\sessions\<session-id>\
├── cache\            --disk-cache-dir
├── cookies\          cookie storage
├── local_data\       localStorage / IndexedDB
├── downloads\        default download dir
├── extensions\       extension data
├── logs\
│   └── renderer.log
└── session.json      session metadata
```

ACLs applied by `filesystem.LockToCurrentUser`:

```
SDDL: O:<SID>G:<SID>D:(A;OICI;FA;;;<SID>)
```

Only the owning user has Full Control. Other local users cannot read
session data.

## Optional Firewall Rules

When enabled, `firewall.SessionPolicy.Apply()` creates two `netsh advfirewall`
rules:

1. **Outbound allow** — permits all outbound traffic from the session
2. **Inbound block** — blocks all non-loopback inbound connections

Rules are tagged with the session ID and removed on session stop.

## Isolation Checker

Run before deploying to verify prerequisites are met:

```powershell
.\bin\sandbox-checker.exe
```

Or programmatically:

```go
report := security.CheckIsolation()
fmt.Println(report.Describe())
```

Verifies that `CreateJobObject` and `OpenProcessToken` succeed.
