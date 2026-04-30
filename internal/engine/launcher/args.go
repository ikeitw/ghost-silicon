// internal/engine/launcher/args.go
// Package launcher — command-line argument builder for the renderer process.
// Constructs the argument list from the session options and config so the
// renderer starts pointed at the correct isolated directories and pipe.
package launcher

import (
	"fmt"
	"path/filepath"
)

// ArgBuilder assembles the renderer command-line arguments.
type ArgBuilder struct {
	args []string
}

// NewArgBuilder creates an empty ArgBuilder.
func NewArgBuilder() *ArgBuilder { return &ArgBuilder{} }

// Add appends a raw argument.
func (b *ArgBuilder) Add(arg string) *ArgBuilder {
	b.args = append(b.args, arg)
	return b
}

// AddFlag appends "--key=value".
func (b *ArgBuilder) AddFlag(key, value string) *ArgBuilder {
	b.args = append(b.args, fmt.Sprintf("--%s=%s", key, value))
	return b
}

// AddSessionArgs appends the standard ghost-silicon session arguments:
// user-data-dir, disk-cache-dir, and the GS pipe environment variable.
// These are recognised by the renderer adapter's injection layer.
func (b *ArgBuilder) AddSessionArgs(userDataDir, cacheDir, pipeName string) *ArgBuilder {
	b.AddFlag("user-data-dir", userDataDir)
	b.AddFlag("disk-cache-dir", cacheDir)
	// Pass the pipe name as a flag so the renderer adapter can pick it up
	// without needing the environment block.
	b.AddFlag("gs-pipe-name", pipeName)
	return b
}

// AddLogArgs sets the renderer log file inside the session log directory.
func (b *ArgBuilder) AddLogArgs(logDir string) *ArgBuilder {
	b.AddFlag("log-file", filepath.Join(logDir, "renderer.log"))
	return b
}

// Extra appends additional arbitrary arguments (from config.engine.args).
func (b *ArgBuilder) Extra(extra []string) *ArgBuilder {
	b.args = append(b.args, extra...)
	return b
}

// Build returns the final argument slice.
func (b *ArgBuilder) Build() []string {
	out := make([]string, len(b.args))
	copy(out, b.args)
	return out
}
