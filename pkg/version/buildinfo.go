package version

import (
	"fmt"
	"runtime"
)

// These variables are injected at build time via -ldflags.
// Example: -ldflags "-X ghost-silicon/pkg/version.CommitHash=abc1234 -X ghost-silicon/pkg/version.BuildTime=2024-01-01T00:00:00Z"
var (
	CommitHash = "unknown"
	BuildTime  = "unknown"
	GoVersion  = runtime.Version()
)

// FullString returns a human-readable version string that includes all build metadata.
func FullString() string {
	return fmt.Sprintf("%s v%s (commit=%s built=%s %s/%s go=%s)",
		AppName, Version, CommitHash, BuildTime,
		runtime.GOOS, runtime.GOARCH, GoVersion,
	)
}

// Short returns a compact version string.
func Short() string {
	return fmt.Sprintf("%s v%s", AppName, Version)
}
