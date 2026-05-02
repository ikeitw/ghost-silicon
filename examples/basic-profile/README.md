# Example: Basic Profile

Demonstrates creating, validating, and saving a ghost-silicon profile.

## Run

```bash
cd examples/basic-profile
go run main.go
```

## What it does

1. Creates a profile from the `windows11-desktop` built-in template
2. Runs field-level validation
3. Runs cross-field consistency checks
4. Saves the profile as a JSON file in the current directory