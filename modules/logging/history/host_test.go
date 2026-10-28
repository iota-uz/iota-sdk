package history

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_Execution(t *testing.T) {
	t.Parallel()
	mcp := Execution{Channel: businessevent.ChannelMCP, OperationID: "op-mcp"}
	tests := []struct {
		name   string
		ctx    context.Context
		header string
		check  func(t *testing.T, e Execution)
	}{
		{"plain request is a UI operation", context.Background(), "req-1", func(t *testing.T, e Execution) {
			t.Helper()
			require.Equal(t, businessevent.ChannelUI, e.Channel)
			require.Equal(t, "req-1", e.RequestID)
			require.NotEmpty(t, e.OperationID)
			require.NotEqual(t, "req-1", e.OperationID)
			require.Nil(t, e.Actor)
		}},
		{"execution set by an outer layer is kept", WithExecution(context.Background(), mcp), "req-2", func(t *testing.T, e Execution) {
			t.Helper()
			require.Equal(t, mcp, e)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var got Execution
			handler := Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got, _ = UseExecution(r.Context())
			}))
			recorder := httptest.NewRecorder()
			recorder.Header().Set("X-Request-Id", tt.header)
			handler.ServeHTTP(recorder, httptest.NewRequestWithContext(tt.ctx, http.MethodGet, "/", nil))
			tt.check(t, got)
		})
	}
}

func TestEntryPointExecutions(t *testing.T) {
	t.Parallel()
	initiator := businessevent.UserActor(testUser(t))
	ui := WithExecution(context.Background(), Execution{Channel: businessevent.ChannelUI, OperationID: "op-ui", RequestID: "req-1"})
	tests := []struct {
		name  string
		ctx   context.Context
		check func(t *testing.T, e Execution)
	}{
		{"system", SystemExecution(context.Background(), "dbctl:seed", "Seed"), func(t *testing.T, e Execution) {
			t.Helper()
			require.Equal(t, businessevent.ChannelJob, e.Channel)
			require.Equal(t, businessevent.ActorSystem, e.Actor.Kind)
			require.Equal(t, "dbctl:seed", e.Actor.ID)
			require.NotEmpty(t, e.OperationID)
		}},
		{"job", JobExecution(context.Background(), "run-1", "reports.export", &initiator), func(t *testing.T, e Execution) {
			t.Helper()
			require.Equal(t, businessevent.ChannelJob, e.Channel)
			require.Equal(t, businessevent.ActorService, e.Actor.Kind)
			require.Equal(t, "job:reports.export", e.Actor.ID)
			require.Equal(t, &initiator, e.Initiator)
			require.Equal(t, "run-1", e.OperationID)
			require.Equal(t, "run-1", e.RunID)
		}},
		{"import inside a request", ImportExecution(ui, "upload-7"), func(t *testing.T, e Execution) {
			t.Helper()
			require.Equal(t, businessevent.ChannelImport, e.Channel)
			require.Nil(t, e.Actor)
			require.Equal(t, "req-1", e.RequestID)
			require.Equal(t, "upload-7", e.RunID)
			require.NotEqual(t, "op-ui", e.OperationID)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e, ok := UseExecution(tt.ctx)
			require.True(t, ok)
			tt.check(t, e)
		})
	}
}
