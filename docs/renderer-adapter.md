# Renderer Adapter

## Default Mode: Embedded WebView2

When `engine.executable` is empty (the default), Ghost-Silicon uses the
system WebView2 runtime embedded directly via go-webview2. No separate
renderer process is launched. The identity polyfill and HTML chrome overlay
are injected via `AddScriptToExecuteOnDocumentCreated`.

This is the recommended mode. The external adapter mechanism below exists for
advanced use cases where a separate Chromium-compatible renderer is needed.

## External Renderer Mode

When `engine.executable` is set, Ghost-Silicon launches that executable as a
separate process and connects to it over the named pipe bridge.

Set in `configs/ghost-silicon.yaml`:

```yaml
engine:
  executable: "C:\\Program Files\\Chromium\\chrome.exe"
  args: ["--disable-extensions"]
```

## Adapter Interface

```go
// pkg/renderer/interface.go

type Adapter interface {
    Start(ctx context.Context, opts StartOptions) (Process, error)
    Name() string
}

type Process interface {
    PID() uint32
    Wait(ctx context.Context) (uint32, error)
    Terminate() error
    IsRunning() bool
}
```

## StartOptions

| Field | Purpose |
|---|---|
| `ProfileID` | Identity profile bound to this session |
| `SessionID` | Unique session identifier |
| `PipeName` | Named pipe path for the IPC bridge |
| `UserDataDir` | Isolated filesystem root for the renderer |
| `CacheDir` | HTTP cache directory |
| `ExtraArgs` | Additional command-line arguments |
| `Env` | Environment block (nil = inherit supervisor env) |

## Built-in Adapters

### Mock Adapter (`internal/engine/adapter`)

Used in tests and developer tooling. Starts immediately, reports
`IsRunning() == true` forever, and `Wait()` returns 0 without blocking.

```go
adapter.Global.Get("mock", "")
```

### Windows Launcher (`internal/engine/launcher`)

Production adapter for external Chromium processes:

1. Builds command-line args via `ArgBuilder`
2. Builds environment block with `GS_SESSION_ID`, `GS_PROFILE_ID`, `GS_PIPE_NAME`
3. Optionally builds a restricted token via `token.Build()`
4. Calls `CreateProcess` / `CreateProcessAsUser` with `CREATE_SUSPENDED`
5. Assigns the process to the Job Object
6. Calls `ResumeThread`

## Registering a Custom Adapter

```go
adapter.Global.Register("chromium", func(executable string) renderer.Adapter {
    return &ChromiumAdapter{executable: executable}
})
```

The supervisor selects an adapter by matching the adapter name against the
`engine.executable` config value. Unrecognised values fall back to the mock
adapter.

## Command-Line Arguments (external renderer)

`ArgBuilder` appends:

```
--user-data-dir=<session-root>
--disk-cache-dir=<session-cache>
--gs-pipe-name=<pipe-name>
--log-file=<session-logs>/renderer.log
[extra args from config]
[extra args from StartOptions]
```

## Crash Restart

The `runtime.Runtime` supervise loop (external renderer only):

1. `Process.Wait(ctx)` — blocks until exit
2. `Restarter.RecordCrash(exitCode)` — increments counter
3. Limit exceeded → return `ErrCrashLimitExceeded`
4. Otherwise → `Restarter.WaitBeforeRestart(ctx)` (exponential backoff)
5. Re-call `adapter.Start()` with the same `StartOptions`
6. Uptime > 30 s since last restart → `Restarter.Reset()`

Configure in `configs/ghost-silicon.yaml`:

```yaml
engine:
  crash_restart_limit: 3
  crash_restart_delay: 2s
```

## Health Monitoring (external renderer)

`internal/engine/health` polls `Process.IsRunning()` every 5 seconds and
emits status changes to a channel. The lifecycle manager drains this channel.

Probes available:
- `ProcessProbe` — checks `IsRunning()` directly
- `HTTPProbe` — HTTP GET to a local health endpoint
