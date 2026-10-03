// Package serrorlog provides bounded error logging attributes and levels.
package serrorlog

import (
	"log/slog"
	"unicode/utf8"

	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
)

// Attribute strings are limited to 256 UTF-8 bytes; traces retain 32 outer frames.
const maxAttributeBytes = 256
const maxTraceOperations = 32

func bounded(value string) string {
	if len(value) <= maxAttributeBytes {
		return value
	}
	value = value[:maxAttributeBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}

func Attributes(err error, requestID string) []slog.Attr {
	attrs := []slog.Attr{slog.String("error.code", serrors.CodeOf(err).String()), slog.String("error.op", bounded(string(serrors.OpOf(err)))), slog.String("error.reason", bounded(string(serrors.ReasonOf(err))))}
	trace := serrors.Trace(err)
	if len(trace) > maxTraceOperations {
		trace = trace[:maxTraceOperations]
	}
	operations := make([]string, len(trace))
	for i, op := range trace {
		operations[i] = bounded(string(op))
	}
	attrs = append(attrs, slog.Any("error.trace", operations))
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", bounded(requestID)))
	}
	return attrs
}

func Level(err error) slog.Level {
	switch serrors.CodeOf(err) {
	case serrors.Canceled:
		return slog.LevelDebug
	case serrors.Invalid, serrors.NotFound, serrors.AlreadyExists, serrors.Conflict, serrors.FailedPrecondition, serrors.PermissionDenied, serrors.Unauthenticated:
		return slog.LevelInfo
	case serrors.RateLimited:
		return slog.LevelWarn
	case serrors.Internal, serrors.Unavailable, serrors.Timeout, serrors.Unimplemented:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
