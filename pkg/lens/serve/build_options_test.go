package serve

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"github.com/stretchr/testify/require"
)

func TestHandlersResourceRejectionsReachObserver(t *testing.T) {
	// Verify public response and telemetry together; directly testing the status helper misses this early-return branch.
	for _, item := range []struct {
		name   string
		cause  error
		status int
	}{{"payload", resource.NewLimitError("entry bytes", 1000, 100), 413}, {"admission", resource.NewLimitError("report_build_queue", 2, 1), 503}} {
		t.Run(item.name, func(t *testing.T) {
			observer := &recordingObserver{}
			handlers, executor, _ := newTestHandlers(t, 0)
			handlers.observer = observer
			executor.executeErrs = map[string]error{"": item.cause}
			response := httptest.NewRecorder()
			handlers.Document(response, httptest.NewRequest(http.MethodGet, "/dash/document", nil))
			require.Equal(t, item.status, response.Code, response.Body.String())
			require.Contains(t, response.Body.String(), "resource_exhausted")
			observed := observer.recorded()
			require.Len(t, observed, 1)
			require.ErrorIs(t, observed[0].err, resource.ErrLimit)
		})
	}
}
