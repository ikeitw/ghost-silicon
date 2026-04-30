package audit

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Writer is a thread-safe append-only file writer for audit records.
// It ensures the parent directory exists and opens the file with O_APPEND
// so concurrent processes can write to the same trail safely.
type Writer struct {
	f *os.File
}

// NewWriter opens path for append writing, creating parent directories
// and the file itself if they do not exist.
func NewWriter(path string) (*Writer, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("audit/writer: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("audit/writer: open %q: %w", path, err)
	}
	return &Writer{f: f}, nil
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) { return w.f.Write(p) }

// Close closes the underlying file.
func (w *Writer) Close() error { return w.f.Close() }

// NewLoggerFromPath is a convenience constructor that creates a Writer and
// wires it into a Logger in one call.
func NewLoggerFromPath(path string) (*Logger, io.Closer, error) {
	w, err := NewWriter(path)
	if err != nil {
		return nil, nil, err
	}
	return New(w), w, nil
}
