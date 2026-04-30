//go:build windows

package filesystem

import "path/filepath"

// RendererFlags returns the command-line flag pairs that point a Chromium-
// compatible renderer at the session's isolated data directories.
// The renderer adapter passes these into LaunchOptions.Args.
func (l *SessionLayout) RendererFlags() []string {
	return []string{
		"--user-data-dir=" + l.Root,
		"--disk-cache-dir=" + l.Cache,
		"--downloads-directory=" + l.Downloads,
		"--log-file=" + filepath.Join(l.Logs, "renderer.log"),
	}
}

// ProfilePath returns the path to the renderer's Default profile folder
// inside the session root, creating it if it does not exist.
func (l *SessionLayout) ProfilePath() string {
	return filepath.Join(l.Root, "Default")
}
