package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobsStub_EnqueueScheduleListCancel(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewJobsStub()
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	enqueueResp := callJobsRPC(t, dispatcher, "tenant-a", "bichat.jobs.enqueue", map[string]any{
		"method": "bichat.task.run",
		"params": map[string]any{"x": 1},
	})
	enqueued := enqueueResp["result"].(map[string]any)
	jobID := enqueued["id"].(string)
	assert.Equal(t, "queued", enqueued["status"])

	scheduleResp := callJobsRPC(t, dispatcher, "tenant-a", "bichat.jobs.schedule", map[string]any{
		"cron":   "0 * * * *",
		"method": "bichat.task.hourly",
		"params": map[string]any{"y": 2},
	})
	scheduled := scheduleResp["result"].(map[string]any)
	assert.Equal(t, "scheduled", scheduled["status"])
	assert.Equal(t, "0 * * * *", scheduled["cron"])

	listResp := callJobsRPC(t, dispatcher, "tenant-a", "bichat.jobs.list", map[string]any{})
	jobs := listResp["result"].([]any)
	require.Len(t, jobs, 2)

	otherTenant := callJobsRPC(t, dispatcher, "tenant-b", "bichat.jobs.list", map[string]any{})
	assert.Len(t, otherTenant["result"].([]any), 0)

	cancelResp := callJobsRPC(t, dispatcher, "tenant-a", "bichat.jobs.cancel", map[string]any{"id": jobID})
	assert.Equal(t, true, cancelResp["result"].(map[string]any)["ok"])
}

func callJobsRPC(t *testing.T, dispatcher *appletenginerpc.Dispatcher, tenantID, method string, params map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{"id": "1", "method": method, "params": params}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
	req.Header.Set("X-Iota-Tenant-Id", tenantID)
	rec := httptest.NewRecorder()
	dispatcher.HandleServerOnlyHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return decoded
}
