# Supervisor Design

## Responsibilities

The supervisor (`internal/app/supervisor`) is the central coordinator. It owns:

- The active identity profile
- The current session ID and directory layout
- The engine runtime (renderer lifecycle)
- The IPC bridge (named pipe + JSON-RPC server)
- The lifecycle manager (ordered start / stop hooks)

## Lifecycle

```
supervisor.Init(ctx)
  │
  ├── loadProfile()         — load from store, or auto-generate
  ├── newSessionID()        — crypto/rand 16-byte hex
  ├── bridge.New()          — wire profile → IPC handlers
  ├── jsonrpc.NewServer()   — register bridge methods
  └── (return)

supervisor.Run(ctx)
  │
  ├── lc.Start(ctx)
  │     ├── "ipc-bridge" start  — open named pipe, begin serving
  │     └── "renderer" start    — launch renderer process
  │
  ├── <block until ctx cancelled>
  │
  └── lc.Stop(shutCtx)    — LIFO order
        ├── "renderer" stop  — Terminate() + Wait()
        └── "ipc-bridge" stop — Close() pipe listener
```

## Profile Resolution

On startup `loadProfile` tries the following in order:

1. Load the profile ID in `identity.default_profile` from the store
2. Load the first profile returned by `store.List()`
3. Auto-generate from `identity.default_template` (if `auto_generate: true`)
4. Return an error (startup aborts)

## Session ID

Generated with `crypto/rand` — 16 random bytes encoded as lowercase hex.
Example: `a3f8c12d4e6b7091a2b3c4d5e6f70819`

The session ID is injected into:
- The renderer's environment block (`GS_SESSION_ID`)
- The renderer's command-line (`--gs-pipe-name`)
- Every audit log event
- The session metadata file

## Worker Goroutines

Two background workers run for the supervisor lifetime:

### Rotation Worker

Polls `RotationState.CheckInterval()` every 10 seconds. When a rotation
fires, calls `bridge.UpdateProfile()` to hot-swap the active profile without
restarting the renderer.

### Health Worker

Drains the health monitor status channel and logs state changes. If the
renderer transitions to `StatusDead` the runtime's crash-restart loop
handles recovery.

## Shutdown Sequence

On `SIGINT` or `SIGTERM`:

1. `cancel()` — propagates cancellation to all goroutines
2. Lifecycle manager runs stop hooks in LIFO order:
    - Renderer: `Terminate()` + `Wait(timeout)`
    - IPC bridge: `Close()` pipe listener
3. Audit event `session.stopped` is written
4. Process exits cleanly

If shutdown exceeds `app.shutdown_timeout` the process calls `os.Exit(1)`.