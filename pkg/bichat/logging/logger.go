package logging

import (
	"context"
	"fmt"
)

// Logger provides structured logging for BiChat components.
// Implementations can use any logging library (slog, zap, logrus, etc.).
type Logger interface {
	// Debug logs a debug-level message with optional fields.
	Debug(ctx context.Context, msg string, fields map[string]any)

	// Info logs an info-level message with optional fields.
	Info(ctx context.Context, msg string, fields map[string]any)

	// Warn logs a warning-level message with optional fields.
	Warn(ctx context.Context, msg string, fields map[string]any)

	// Error logs an error-level message with optional fields.
	Error(ctx context.Context, msg string, fields map[string]any)

	// With returns a new logger with the given fields added to all log entries.
	With(fields map[string]any) Logger
}

// NoOpLogger is a logger that does nothing.
// Useful as a default when logging is disabled.
type NoOpLogger struct{}

// NewNoOpLogger creates a no-op logger.
func NewNoOpLogger() Logger {
	return &NoOpLogger{}
}

// Debug does nothing.
func (l *NoOpLogger) Debug(ctx context.Context, msg string, fields map[string]any) {}

// Info does nothing.
func (l *NoOpLogger) Info(ctx context.Context, msg string, fields map[string]any) {}

// Warn does nothing.
func (l *NoOpLogger) Warn(ctx context.Context, msg string, fields map[string]any) {}

// Error does nothing.
func (l *NoOpLogger) Error(ctx context.Context, msg string, fields map[string]any) {}

// With returns the same no-op logger.
func (l *NoOpLogger) With(fields map[string]any) Logger {
	return l
}

// StdLogger logs to stdout/stderr using fmt package.
// Suitable for development and simple deployments.
type StdLogger struct {
	fields map[string]any
}

// NewStdLogger creates a logger that writes to stdout/stderr.
func NewStdLogger() Logger {
	return &StdLogger{
		fields: make(map[string]any),
	}
}

// Debug logs to stdout.
func (l *StdLogger) Debug(ctx context.Context, msg string, fields map[string]any) {
	l.log("DEBUG", msg, fields)
}

// Info logs to stdout.
func (l *StdLogger) Info(ctx context.Context, msg string, fields map[string]any) {
	l.log("INFO", msg, fields)
}

// Warn logs to stdout.
func (l *StdLogger) Warn(ctx context.Context, msg string, fields map[string]any) {
	l.log("WARN", msg, fields)
}

// Error logs to stderr.
func (l *StdLogger) Error(ctx context.Context, msg string, fields map[string]any) {
	l.log("ERROR", msg, fields)
}

// With returns a new logger with additional fields.
func (l *StdLogger) With(fields map[string]any) Logger {
	newFields := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		newFields[k] = v
	}
	for k, v := range fields {
		newFields[k] = v
	}
	return &StdLogger{fields: newFields}
}

// log formats and prints the log message.
func (l *StdLogger) log(level, msg string, fields map[string]any) {
	// Merge instance fields with message fields
	allFields := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		allFields[k] = v
	}
	for k, v := range fields {
		allFields[k] = v
	}

	// Format: [LEVEL] message key=value key=value
	fmt.Printf("[%s] %s", level, msg)
	for k, v := range allFields {
		fmt.Printf(" %s=%v", k, v)
	}
	fmt.Println()
}
