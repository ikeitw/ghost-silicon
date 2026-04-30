// Package logging provides the structured logger used throughout ghost-silicon.
// It wraps the standard library log/slog so the rest of the codebase never
// imports slog directly — giving us one place to change backends.
package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// Logger is the ghost-silicon structured logger.
type Logger struct {
	inner *slog.Logger
}

// New builds a Logger from the given options.
// If opts is nil, Info-level text output to stderr is used.
func New(opts *Options) (*Logger, error) {
	if opts == nil {
		opts = &Options{Level: "info", Format: "text"}
	}

	level, err := parseLevel(opts.Level)
	if err != nil {
		return nil, err
	}

	writers := []io.Writer{os.Stderr}
	if opts.OutputFile != "" {
		f, err := openLogFile(opts.OutputFile)
		if err != nil {
			return nil, err
		}
		writers = append(writers, f)
	}

	out := io.MultiWriter(writers...)
	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch opts.Format {
	case "json":
		handler = slog.NewJSONHandler(out, handlerOpts)
	default:
		handler = slog.NewTextHandler(out, handlerOpts)
	}

	return &Logger{inner: slog.New(handler)}, nil
}

// Options configures the logger.
type Options struct {
	Level      string // "debug" | "info" | "warn" | "error"
	Format     string // "text" | "json"
	OutputFile string // optional additional log file path
}

// With returns a child logger with the given attributes attached.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{inner: l.inner.With(args...)}
}

// WithComponent returns a child logger with a "component" key attached.
func (l *Logger) WithComponent(name string) *Logger {
	return l.With("component", name)
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) { l.inner.Debug(msg, args...) }

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) { l.inner.Info(msg, args...) }

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) { l.inner.Warn(msg, args...) }

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) { l.inner.Error(msg, args...) }

// DebugCtx logs at DEBUG level with context.
func (l *Logger) DebugCtx(ctx context.Context, msg string, args ...any) {
	l.inner.DebugContext(ctx, msg, args...)
}

// InfoCtx logs at INFO level with context.
func (l *Logger) InfoCtx(ctx context.Context, msg string, args ...any) {
	l.inner.InfoContext(ctx, msg, args...)
}

// WarnCtx logs at WARN level with context.
func (l *Logger) WarnCtx(ctx context.Context, msg string, args ...any) {
	l.inner.WarnContext(ctx, msg, args...)
}

// ErrorCtx logs at ERROR level with context.
func (l *Logger) ErrorCtx(ctx context.Context, msg string, args ...any) {
	l.inner.ErrorContext(ctx, msg, args...)
}

// Nop returns a logger that discards all output.  Useful in tests.
func Nop() *Logger {
	return &Logger{inner: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func parseLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, nil
	}
}

func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}
