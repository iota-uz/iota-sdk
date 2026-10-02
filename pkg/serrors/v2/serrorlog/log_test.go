package serrorlog_test

import (
	"errors"
	"log/slog"
	"testing"

	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorlog"
	"github.com/stretchr/testify/require"
)

func TestAttributesExcludeDiagnostics(t *testing.T) {
	err := serrors.NewUnavailable("password=secret").WithOp("dependency.Fetch").WithReason("offline").WithCause(errors.New("secret cause")).WithMeta(map[string]serrors.Value{"token": serrors.Text("secret")})
	attrs := serrorlog.Attributes(err, "request-123")
	values := map[string]string{}
	for _, a := range attrs {
		values[a.Key] = a.Value.String()
	}
	require.Equal(t, map[string]string{"error.code": "unavailable", "error.op": "dependency.Fetch", "error.reason": "offline", "request_id": "request-123", "error.trace": "[dependency.Fetch]"}, values)
	require.Len(t, serrorlog.Attributes(err, ""), 4)
}
func TestLevelDistinguishesClientCancellationAndDependencyFailure(t *testing.T) {
	for _, tc := range []struct {
		code  serrors.Code
		level slog.Level
	}{{serrors.Canceled, slog.LevelDebug}, {serrors.Invalid, slog.LevelInfo}, {serrors.RateLimited, slog.LevelWarn}, {serrors.Unavailable, slog.LevelError}} {
		require.Equal(t, tc.level, serrorlog.Level(serrors.New(tc.code, "")))
	}
}
