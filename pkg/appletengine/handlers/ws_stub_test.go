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

type wsBroadcasterMock struct {
	appletID     string
	connectionID string
	payload      any
}

func (m *wsBroadcasterMock) Send(appletID, connectionID string, payload any) error {
	m.appletID = appletID
	m.connectionID = connectionID
	m.payload = payload
	return nil
}

func TestWSStub_Send(t *testing.T) {
	t.Parallel()

	registry := appletenginerpc.NewRegistry()
	mock := &wsBroadcasterMock{}
	stub := NewWSStub(mock)
	require.NoError(t, stub.Register(registry, "bichat"))
	dispatcher := appletenginerpc.NewDispatcher(registry, nil, logrus.New())

	payload := map[string]any{
		"id":     "1",
		"method": "bichat.ws.send",
		"params": map[string]any{
			"connectionId": "conn-1",
			"data":         map[string]any{"x": 1},
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	dispatcher.HandleServerOnlyHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	assert.Equal(t, "bichat", mock.appletID)
	assert.Equal(t, "conn-1", mock.connectionID)
	assert.Equal(t, map[string]any{"x": float64(1)}, mock.payload)
}

func TestWSStub_RequiresBroadcaster(t *testing.T) {
	t.Parallel()
	registry := appletenginerpc.NewRegistry()
	err := NewWSStub(nil).Register(registry, "bichat")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ws broadcaster is required")
}
