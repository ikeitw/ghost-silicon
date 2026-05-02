# Example: Profile Switching

Demonstrates loading profiles from disk and rotating between them using the
RotationState engine.

## Run

```bash
cd examples/profile-switching
go run main.go
```

## What it does

1. Loads all profiles from the `profiles/` directory
2. Sets up an `on_session` rotation policy
3. Simulates three session starts and shows the profile changing each time
4. Prints the final active profile details