package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	appletenginerpc "github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticSecretsStore struct {
	values map[string]string
}

func (s *staticSecretsStore) Get(_ context.Context, appletName, name string) (string, bool, error) {
	value, ok := s.values[appletName+"::"+name]
	return value, ok, nil
}

func TestSecretsStub_Get(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewSecretsStubWithStore(&staticSecretsStore{
		values: map[string]string{
			"bichat::openai_api_key": "secret-value",
		},
	})
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	resp := callSecretsRPC(t, dispatcher, "bichat.secrets.get", map[string]any{"name": "openai_api_key"})
	require.NotNil(t, resp["result"])
	result := resp["result"].(map[string]any)
	assert.Equal(t, "secret-value", result["value"])
}

func TestSecretsStub_NotFound(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	stub := NewSecretsStubWithStore(&staticSecretsStore{values: map[string]string{}})
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	resp := callSecretsRPC(t, dispatcher, "bichat.secrets.get", map[string]any{"name": "missing"})
	require.NotNil(t, resp["error"])
	assert.Nil(t, resp["result"])
}

func TestNormalizeSecretSegment(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "BICHAT", normalizeSecretSegment("bichat"))
	assert.Equal(t, "OPENAI_API_KEY", normalizeSecretSegment("openai.api-key"))
}

func callSecretsRPC(t *testing.T, dispatcher *appletenginerpc.Dispatcher, method string, params map[string]any) map[string]any {
	t.Helper()
	payload := map[string]any{"id": "1", "method": method, "params": params}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
	req.Header.Set("X-Iota-Tenant-Id", "tenant-a")
	rec := httptest.NewRecorder()
	dispatcher.HandleServerOnlyHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return decoded
}
