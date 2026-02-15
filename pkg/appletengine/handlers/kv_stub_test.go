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

func TestKVStub_CRUDAndTenantIsolation(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewKVStub()
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	setResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.kv.set", map[string]any{"key": "k1", "value": "v1"})
	assert.Equal(t, "2.0", setResp["jsonrpc"])
	assert.Equal(t, true, setResp["result"].(map[string]any)["ok"])

	getResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.kv.get", map[string]any{"key": "k1"})
	assert.Equal(t, "v1", getResp["result"])

	otherTenantGet := callServerRPC(t, dispatcher, "tenant-b", "bichat.kv.get", map[string]any{"key": "k1"})
	assert.Nil(t, otherTenantGet["result"])

	mgetResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.kv.mget", map[string]any{"keys": []string{"k1", "missing"}})
	values := mgetResp["result"].([]any)
	require.Len(t, values, 2)
	assert.Equal(t, "v1", values[0])
	assert.Nil(t, values[1])

	delResp := callServerRPC(t, dispatcher, "tenant-a", "bichat.kv.del", map[string]any{"key": "k1"})
	assert.Equal(t, true, delResp["result"])

	getAfterDelete := callServerRPC(t, dispatcher, "tenant-a", "bichat.kv.get", map[string]any{"key": "k1"})
	assert.Nil(t, getAfterDelete["result"])
}

func callServerRPC(t *testing.T, dispatcher *appletenginerpc.Dispatcher, tenantID, method string, params map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{
		"id":     "1",
		"method": method,
		"params": params,
	}
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
