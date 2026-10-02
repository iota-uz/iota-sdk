// Package serrorlog provides bounded structured error attributes.
package serrorlog

import (
	"log/slog"

	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
)

func Attributes(err error, requestID string) []slog.Attr {
	attrs := []slog.Attr{slog.String("error.code", serrors.CodeOf(err).String()), slog.String("error.op", string(serrors.OpOf(err))), slog.String("error.reason", string(serrors.ReasonOf(err)))}
	trace := serrors.Trace(err)
	operations := make([]string, len(trace))
	for i, op := range trace {
		operations[i] = string(op)
	}
	attrs = append(attrs, slog.Any("error.trace", operations))
	if requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
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
