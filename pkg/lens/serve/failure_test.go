package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// erroringPanelExecutor fails one deferred panel with a caller-supplied cause,
// which is the only part of a real execution failure the wire contract keeps.
type erroringPanelExecutor struct {
	base  *lensruntime.Runtime
	panel string
	cause error
}

func (e erroringPanelExecutor) Execute(
	ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope,
) (*lensruntime.DashboardResult, error) {
	result, err := e.base.Execute(ctx, spec, req, scope)
	if err != nil || scope.MetadataOnly || len(scope.PanelIDs) != 1 || scope.PanelIDs[0] != e.panel {
		return result, err
	}
	result.Panels[e.panel].Error = e.cause
	result.Panels[e.panel].Frames = nil
	return result, nil
}

// A failed panel tells the browser what kind of failure it was, so the runtime
// can say something a reader can act on instead of one sentence for every
// cause. The developer text stays off the wire.
func TestHandlers_PanelFailureCarriesReason(t *testing.T) {
	t.Parallel()
	for name, testCase := range map[string]struct {
		cause    error
		expected document.FailureReason
	}{
		"deadline": {
			cause:    fmt.Errorf("analytics.repository.trends: %w", context.DeadlineExceeded),
			expected: document.FailureTimeout,
		},
		"stringified deadline": {
			cause:    errors.New("lens/runtime.MemoizeJSON: analytics.repository.trends: timeout: context deadline exceeded"),
			expected: document.FailureTimeout,
		},
		"cancellation": {
			cause:    fmt.Errorf("analytics.repository.trends: %w", context.Canceled),
			expected: document.FailureCanceled,
		},
		"anything else": {
			cause:    errors.New("column \"premium\" does not exist"),
			expected: document.FailureUnknown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fastFrames := testFrames(t, "fast", 10)
			slowFrames := testFrames(t, "slow", 20)
			spec := lens.DashboardSpec{
				ID: "failures", Title: "Failures",
				Rows: []lens.RowSpec{{Panels: []panel.Spec{
					panel.Stat("fast", "Fast", "fast-data").Terminal().Build(),
					panel.Stat("slow", "Slow", "slow-data").Terminal().Build(),
				}}},
				Datasets: []lens.DatasetSpec{staticDataset("fast-data", fastFrames), staticDataset("slow-data", slowFrames)},
			}
			handlers, err := New(Config{
				Spec: spec,
				Engine: erroringPanelExecutor{
					base: lensruntime.New(lensruntime.Options{}), panel: "slow", cause: testCase.cause,
				},
				Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash", Progressive: true,
				Observer: &recordingObserver{},
				Request: func(r *http.Request) lensruntime.Request {
					return lensruntime.Request{Locale: "en", DataScope: "tenant:test", Request: r.URL.Query()}
				},
			})
			require.NoError(t, err)

			doc := requestDocument(t, handlers, "/dash/document")
			payload, err := json.Marshal(PanelBatchRequest{
				SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "slow"}},
			})
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			handlers.Panel(recorder, httptest.NewRequest(
				http.MethodPost, "/dash/lens/panel?tenant=test", bytes.NewReader(payload),
			))
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

			result := decodePanelBatchStream(t, recorder.Body.Bytes()).Panels["slow"]
			require.NotNil(t, result.Error)
			require.Equal(t, testCase.expected, result.Error.Reason)
			require.Equal(t, document.QueryErrorInternal, result.Error.Error)
			require.Equal(t, "panel execution failed", result.Error.Message, "the cause must never reach the browser")
		})
	}
}

// A rejected request never ran, so it has no failure to classify and the
// runtime keeps printing its generic sentence for it.
func TestHandlers_RejectedPanelRequestHasNoReason(t *testing.T) {
	t.Parallel()
	frames := testFrames(t, "value", 10)
	spec := lens.DashboardSpec{
		ID: "rejections", Title: "Rejections",
		Rows:     []lens.RowSpec{{Panels: []panel.Spec{panel.Stat("value", "Value", "value-data").Terminal().Build()}}},
		Datasets: []lens.DatasetSpec{staticDataset("value-data", frames)},
	}
	handlers, err := New(Config{
		Spec: spec, Engine: lensruntime.New(lensruntime.Options{}),
		Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash", Progressive: true,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:test", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)

	doc := requestDocument(t, handlers, "/dash/document")
	payload, err := json.Marshal(PanelBatchRequest{
		SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "value", Page: -1}},
	})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	handlers.Panel(recorder, httptest.NewRequest(
		http.MethodPost, "/dash/lens/panel?tenant=test", bytes.NewReader(payload),
	))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	result := decodePanelBatchStream(t, recorder.Body.Bytes()).Panels["value"]
	require.NotNil(t, result.Error)
	require.Equal(t, document.QueryErrorBadRequest, result.Error.Error)
	require.Empty(t, result.Error.Reason)
}
