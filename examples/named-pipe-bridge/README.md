# Example: Named Pipe Bridge

Demonstrates how the JSON-RPC bridge wires a profile to handler methods
that the renderer calls over the named pipe IPC channel.

## Run

```bash
cd examples/named-pipe-bridge
go run main.go handler.go
```

## What it does

1. Creates a Windows 11 desktop profile
2. Registers all standard bridge methods on a JSON-RPC server
3. Calls each method directly and prints the profile-backed responses
4. Shows how to add custom application-specific handlers