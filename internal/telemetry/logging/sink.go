package logging

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// FileSink is a thread-safe io.Writer that writes to a log file.
// It does not rotate — use an external log manager (e.g. Windows Task
// Scheduler or logrotate on Linux) for rotation.
type FileSink struct {
	mu   sync.Mutex
	file *os.File
	path string
}

// NewFileSink opens or creates the log file at path.
func NewFileSink(path string) (*FileSink, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("logging/sink: open %q: %w", path, err)
	}
	return &FileSink{file: f, path: path}, nil
}

// Write implements io.Writer.
func (s *FileSink) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Write(p)
}

// Close flushes and closes the underlying file.
func (s *FileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.file.Close()
}

// Path returns the file path this sink writes to.
func (s *FileSink) Path() string { return s.path }

// MultiSink fans out writes to multiple io.Writers.
type MultiSink struct {
	writers []io.Writer
}

// NewMultiSink wraps several writers into one.
func NewMultiSink(writers ...io.Writer) *MultiSink {
	return &MultiSink{writers: writers}
}

// Write writes p to all underlying writers.  Returns the first error
// encountered, but always attempts all writers.
func (m *MultiSink) Write(p []byte) (n int, err error) {
	for _, w := range m.writers {
		if wn, werr := w.Write(p); werr != nil && err == nil {
			err = werr
			n = wn
		}
	}
	if err == nil {
		n = len(p)
	}
	return n, err
}
