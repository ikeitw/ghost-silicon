# Renderer Adapter

Ghost-Silicon does not implement a browser engine. Instead it wraps any
Chromium-compatible renderer via the `renderer.Adapter` interface.

## Interface

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

| Field         | Purpose                                              |
|---------------|------------------------------------------------------|
| `ProfileID`   | Identity profile bound to this session               |
| `SessionID`   | Unique session identifier                            |
| `PipeName`    | Named pipe path for the IPC bridge                   |
| `UserDataDir` | Isolated filesystem root for the renderer            |
| `CacheDir`    | HTTP cache directory                                 |
| `ExtraArgs`   | Additional command-line arguments                    |
| `Env`         | Environment block (nil = inherit supervisor env)     |

## Built-in Adapters

### Mock Adapter (`internal/engine/adapter`)

Used in tests and developer tooling. Starts immediately, reports
`IsRunning() == true` forever, and `Wait()` returns 0 without blocking.

Register name: `"mock"`

```go
adapter.Global.Get("mock", "")
```

### Windows Launcher (`internal/engine/launcher`)

The production Windows adapter. Calls `process.Launch()` which:

1. Builds the command-line arguments via `ArgBuilder`
2. Builds the environment block via `EnvBuilder` with `GS_SESSION_ID`,
   `GS_PROFILE_ID`, and `GS_PIPE_NAME` injected
3. Optionally builds a restricted token via `token.Build()`
4. Calls `CreateProcess` / `CreateProcessAsUser` with `CREATE_SUSPENDED`
5. Assigns the process to the Job Object
6. Calls `ResumeThread` to start execution

## Registering a Custom Adapter

```go
adapter.Global.Register("chromium", func(executable string) renderer.Adapter {
    return &ChromiumAdapter{executable: executable}
})
```

Then set in config:

```yaml
engine:
  executable: "C:\\Program Files\\Chromium\\chrome.exe"
```

The supervisor selects the adapter by matching the config `engine.executable`
field against registered adapter names. If no match is found it falls back to
the mock adapter.

## Command-Line Arguments

The `ArgBuilder` in `internal/engine/launcher/args.go` appends:

```
--user-data-dir=<session-root>
--disk-cache-dir=<session-cache>
--gs-pipe-name=<pipe-name>
--log-file=<session-logs>/renderer.log
[extra args from config]
[extra args from StartOptions]
```

Any Chromium-compatible renderer that reads `--user-data-dir` and
`--disk-cache-dir` will honour session isolation automatically.

The `--gs-pipe-name` flag is a ghost-silicon extension. The renderer adapter
layer in the renderer reads it at startup and connects to the bridge.

## Health Monitoring

The engine health monitor (`internal/engine/health`) polls
`Process.IsRunning()` every 5 seconds (configurable) and emits status changes
to a channel. The supervisor drains this channel in the health worker goroutine.

Probes available:
- `ProcessProbe` — checks `IsRunning()` directly
- `HTTPProbe` — performs HTTP GET to a local health endpoint

## Crash Restart

The `runtime.Runtime` supervise loop:

1. Calls `Process.Wait(ctx)` — blocks until exit
2. Calls `Restarter.RecordCrash(exitCode)` — increments counter
3. If limit exceeded: returns `ErrCrashLimitExceeded`
4. Otherwise: calls `Restarter.WaitBeforeRestart(ctx)` (exponential backoff)
5. Re-calls `adapter.Start()` with the same `StartOptions`
6. If uptime > 30s since last restart: calls `Restarter.Reset()` (clears count)