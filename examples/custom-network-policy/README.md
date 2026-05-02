# Example: Custom Network Policy

Demonstrates building a transport with host-level ACL rules.

## Run

```bash
cd examples/custom-network-policy
go run main.go
```

## What it does

1. Starts two local HTTP test servers
2. Builds a network policy that blocks one host
3. Shows that blocked requests are rejected by the transport
4. Shows that allowed requests succeed