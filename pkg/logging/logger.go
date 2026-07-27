package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// Level wraps slog.Level for our logging levels.
type Level slog.Level

const (
	LevelDebug Level = Level(slog.LevelDebug)
	LevelInfo  Level = Level(slog.LevelInfo)
	LevelWarn  Level = Level(slog.LevelWarn)
	LevelError Level = Level(slog.LevelError)
)

// Logger provides structured logging with context support.
type Logger struct {
	inner *slog.Logger
}

// New creates a new Logger.
func New(service string, level Level, w io.Writer) *Logger {
	if w == nil {
		w = os.Stdout
	}

	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.Level(level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.String("timestamp", time.Now().UTC().Format(time.RFC3339Nano))
			}
			if a.Key == slog.SourceKey {
				return slog.Attr{}
			}
			return a
		},
	})

	logger := slog.New(handler).With(
		slog.String("service", service),
	)

	return &Logger{inner: logger}
}

// NewNop creates a no-op logger for testing.
func NewNop() *Logger {
	return &Logger{inner: slog.New(slog.NewJSONHandler(io.Discard, nil))}
}

// Debug logs at debug level.
func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

// Info logs at info level.
func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Warn logs at warn level.
func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

// Error logs at error level with stack trace.
func (l *Logger) Error(msg string, err error, args ...any) {
	pc, file, line, ok := runtime.Caller(1)
	caller := "unknown"
	if ok {
		fn := runtime.FuncForPC(pc)
		if fn != nil {
			caller = fmt.Sprintf("%s (%s:%d)", fn.Name(), file, line)
		}
	}

	errAttrs := []any{
		slog.String("error", err.Error()),
		slog.String("caller", caller),
	}
	args = append(errAttrs, args...)
	l.inner.Error(msg, args...)
}

// With returns a child logger with additional fields.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{inner: l.inner.With(args...)}
}

// WithRequestID adds a request ID to the logger context.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.With(slog.String("request_id", requestID))
}

// WithComponent adds a component name to the logger context.
func (l *Logger) WithComponent(component string) *Logger {
	return l.With(slog.String("component", component))
}

// CtxLogger wraps a context-aware logger.
type CtxLogger struct {
	*Logger
}

// FromContext retrieves a logger from context or returns the default.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	return defaultLogger
}

// WithContext stores a logger in context.
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

type contextKey string

const loggerKey contextKey = "logger"

var defaultLogger = New("sentinel-cnapp", LevelInfo, os.Stdout)
