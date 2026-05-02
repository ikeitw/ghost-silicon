# Example: Isolated Renderer (Windows)

Shows how ghost-silicon configures Windows process isolation before launching
a renderer — Job Objects, restricted tokens, and per-session directories.

## Run

```bash
cd examples/isolated-renderer-windows
go run main.go
```

## What it does

1. Probes the host for available isolation primitives
2. Builds a sandbox policy with memory and CPU caps
3. Loads a Windows 11 identity profile
4. Prints the full set of launch options that would be passed to the launcher