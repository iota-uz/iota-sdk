package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilesStub_StoreGetDeleteAndScopeIsolation(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewFilesStub()
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	payload := base64.StdEncoding.EncodeToString([]byte("hello files"))
	storeResp := callFilesRPC(t, dispatcher, "tenant-a", "bichat.files.store", map[string]any{
		"name":        "sample.txt",
		"contentType": "text/plain",
		"dataBase64":  payload,
	})
	require.NotNil(t, storeResp["result"])
	stored := storeResp["result"].(map[string]any)
	fileID := stored["id"].(string)
	assert.Equal(t, "sample.txt", stored["name"])

	getResp := callFilesRPC(t, dispatcher, "tenant-a", "bichat.files.get", map[string]any{"id": fileID})
	require.NotNil(t, getResp["result"])
	assert.Equal(t, fileID, getResp["result"].(map[string]any)["id"])

	otherTenant := callFilesRPC(t, dispatcher, "tenant-b", "bichat.files.get", map[string]any{"id": fileID})
	assert.Nil(t, otherTenant["result"])

	deleteResp := callFilesRPC(t, dispatcher, "tenant-a", "bichat.files.delete", map[string]any{"id": fileID})
	assert.Equal(t, true, deleteResp["result"])

	afterDelete := callFilesRPC(t, dispatcher, "tenant-a", "bichat.files.get", map[string]any{"id": fileID})
	assert.Nil(t, afterDelete["result"])
}

func TestFilesStub_StoreRejectsInvalidBase64(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewFilesStub()
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	resp := callFilesRPC(t, dispatcher, "tenant-a", "bichat.files.store", map[string]any{
		"name":       "sample.txt",
		"dataBase64": "%%%invalid%%%",
	})
	require.NotNil(t, resp["error"])
	assert.Nil(t, resp["result"])
}

func callFilesRPC(t *testing.T, dispatcher *appletenginerpc.Dispatcher, tenantID, method string, params map[string]any) map[string]any {
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
