# Process Isolation

Ghost-Silicon uses Windows-native isolation primitives to restrict what the
renderer process can do and see.

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

## Job Object Details

Created with `CreateJobObject` in `internal/platform/windows/jobobject`.

Flags applied:
- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — kills all processes in the job
  when the last handle to the job is closed (i.e. when the supervisor exits)
- `JOB_OBJECT_LIMIT_JOB_MEMORY` — optional memory cap in bytes
- `JOB_OBJECT_CPU_RATE_CONTROL_ENABLE | JOB_OBJECT_CPU_RATE_CONTROL_HARD_CAP`
  — optional CPU rate cap (units: 1/100th of a percent)

The renderer is launched with `CREATE_SUSPENDED`, assigned to the Job Object,
then resumed via `ResumeThread`. This guarantees the process cannot escape the
job between creation and assignment.

## Token Restriction

Created with `DuplicateTokenEx` + `AdjustTokenPrivileges` + `SetTokenInformation`.

Privileges removed (`SE_PRIVILEGE_REMOVED`):

| Privilege                    | Why removed                             |
|------------------------------|-----------------------------------------|
| SeDebugPrivilege             | Cannot attach debugger to other processes |
| SeLoadDriverPrivilege        | Cannot load kernel drivers              |
| SeTcbPrivilege               | Cannot act as OS                        |
| SeBackupPrivilege            | Cannot bypass file ACLs                 |
| SeRestorePrivilege           | Cannot bypass file ACLs                 |
| SeCreateTokenPrivilege       | Cannot forge tokens                     |
| SeTakeOwnershipPrivilege     | Cannot seize ownership of objects       |
| SeAssignPrimaryTokenPrivilege| Cannot swap process tokens              |
| SeImpersonatePrivilege       | Cannot impersonate other users          |
| SeCreateGlobalPrivilege      | Cannot create global kernel objects     |

Mandatory integrity level is set to Medium by default. Low integrity is
available for maximum isolation (some features may break at Low).

## Per-Session Filesystem

Directory tree created by `filesystem.CreateSessionLayout`:

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

When enabled, `firewall.SessionPolicy.Apply()` creates two rules via netsh:

1. **Outbound allow** — permits all outbound traffic from the session
2. **Inbound block** — blocks all non-loopback inbound connections

Rules are tagged with the session ID and removed on session stop.

## Linux / macOS (Phase 1)

On Linux and macOS, Phase 1 uses a plain `exec.Cmd` launch with no additional
isolation. Phase 2 will add:

- Linux: UTS + net + mnt + user + PID namespaces via `clone(2)`, cgroup v2
  limits, seccomp-BPF filter
- macOS: `sandbox-exec` with a deny-default Seatbelt profile

## Isolation Check

Run the sandbox-checker tool before deploying:

```powershell
.\bin\sandbox-checker.exe
```

Or programmatically:

```go
report := security.CheckIsolation()
fmt.Println(report.Describe())
```

This verifies that `CreateJobObject` and `OpenProcessToken` succeed, which
are the two prerequisites for the full isolation stack.