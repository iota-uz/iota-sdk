package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/applets"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatcher_ValidSingleRequest(t *testing.T) {
	t.Parallel()

	dispatcher := testDispatcherWithPublicMethod(t, "bichat.echo", func(_ context.Context, params json.RawMessage) (any, error) {
		var p map[string]any
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return map[string]any{"echo": p["value"]}, nil
	})

	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"req-1","method":"bichat.echo","params":{"value":"ok"}}`)
	require.Equal(t, http.StatusOK, resp.Code)

	decoded := decodeObject(t, resp.Body.Bytes())
	assert.Equal(t, "2.0", decoded["jsonrpc"])
	assert.Equal(t, "req-1", decoded["id"])
	result := decoded["result"].(map[string]any)
	assert.Equal(t, "ok", result["echo"])
}

func TestDispatcher_ValidBatchRequest(t *testing.T) {
	t.Parallel()

	dispatcher := testDispatcherWithPublicMethod(t, "bichat.echo", func(_ context.Context, params json.RawMessage) (any, error) {
		var p map[string]any
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		return p, nil
	})

	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `[
		{"id":"1","method":"bichat.echo","params":{"x":1}},
		{"id":"2","method":"bichat.echo","params":{"x":2}}
	]`)
	require.Equal(t, http.StatusOK, resp.Code)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &decoded))
	require.Len(t, decoded, 2)

	assert.Equal(t, "1", decoded[0]["id"])
	assert.Equal(t, "2", decoded[1]["id"])
	assert.InDelta(t, 1.0, decoded[0]["result"].(map[string]any)["x"].(float64), 0.0)
	assert.InDelta(t, 2.0, decoded[1]["result"].(map[string]any)["x"].(float64), 0.0)
}

func TestDispatcher_MethodNotFound(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(NewRegistry(), nil, logrus.New())
	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"m1","method":"bichat.missing","params":{}}`)
	require.Equal(t, http.StatusOK, resp.Code)

	decoded := decodeObject(t, resp.Body.Bytes())
	errorObj := decoded["error"].(map[string]any)
	assert.InDelta(t, -32601.0, errorObj["code"].(float64), 0.0)
	assert.Equal(t, "m1", decoded["id"])
}

func TestDispatcher_InvalidRequestPayload(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(NewRegistry(), nil, logrus.New())
	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"x"`)
	require.Equal(t, http.StatusBadRequest, resp.Code)

	decoded := decodeObject(t, resp.Body.Bytes())
	errorObj := decoded["error"].(map[string]any)
	assert.InDelta(t, -32600.0, errorObj["code"].(float64), 0.0)
}

func TestDispatcher_InvalidRequestMethodAndRequestIDPassthrough(t *testing.T) {
	t.Parallel()

	dispatcher := NewDispatcher(NewRegistry(), nil, logrus.New())
	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":42,"method":"","params":{}}`)
	require.Equal(t, http.StatusOK, resp.Code)

	decoded := decodeObject(t, resp.Body.Bytes())
	errorObj := decoded["error"].(map[string]any)
	assert.InDelta(t, -32600.0, errorObj["code"].(float64), 0.0)
	assert.InDelta(t, 42.0, decoded["id"].(float64), 0.0)
}

type bunCallerStub struct {
	called     bool
	lastApplet string
	lastMethod string
}

func (s *bunCallerStub) CallPublicMethod(_ context.Context, appletID, method string, _ json.RawMessage, _ http.Header) (any, error) {
	s.called = true
	s.lastApplet = appletID
	s.lastMethod = method
	return map[string]any{"via": "bun"}, nil
}

func TestDispatcher_PublicTargetBun_UsesBunCaller(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicWithTarget("bichat", "bichat.ping", MethodTargetBun, applets.RPCMethod{
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"via": "go"}, nil
		},
	}, nil))

	dispatcher := NewDispatcher(registry, nil, logrus.New())
	bunCaller := &bunCallerStub{}
	dispatcher.SetBunPublicCaller(bunCaller)

	resp := doRPCRequest(t, dispatcher.HandlePublicHTTP, `{"id":"1","method":"bichat.ping","params":{}}`)
	require.Equal(t, http.StatusOK, resp.Code)
	decoded := decodeObject(t, resp.Body.Bytes())
	result := decoded["result"].(map[string]any)
	assert.Equal(t, "bun", result["via"])
	assert.True(t, bunCaller.called)
	assert.Equal(t, "bichat", bunCaller.lastApplet)
	assert.Equal(t, "bichat.ping", bunCaller.lastMethod)
}

func TestDispatcher_InternalTransport_CallsGoHandlerEvenWhenTargetBun(t *testing.T) {
	t.Parallel()

	registry := NewRegistry()
	require.NoError(t, registry.RegisterPublicWithTarget("bichat", "bichat.ping", MethodTargetBun, applets.RPCMethod{
		Handler: func(_ context.Context, _ json.RawMessage) (any, error) {
			return map[string]any{"via": "go"}, nil
		},
	}, nil))

	dispatcher := NewDispatcher(registry, nil, logrus.New())
	bunCaller := &bunCallerStub{}
	dispatcher.SetBunPublicCaller(bunCaller)

	resp := doRPCRequest(t, dispatcher.HandleServerOnlyHTTP, `{"id":"1","method":"bichat.ping","params":{}}`)
	require.Equal(t, http.StatusOK, resp.Code)
	decoded := decodeObject(t, resp.Body.Bytes())
	result := decoded["result"].(map[string]any)
	assert.Equal(t, "go", result["via"])
	assert.False(t, bunCaller.called)
}

func testDispatcherWithPublicMethod(t *testing.T, methodName string, handler func(context.Context, json.RawMessage) (any, error)) *Dispatcher {
	t.Helper()
	registry := NewRegistry()
	err := registry.RegisterPublic("bichat", methodName, applets.RPCMethod{Handler: handler}, nil)
	require.NoError(t, err)
	return NewDispatcher(registry, nil, logrus.New())
}

func doRPCRequest(t *testing.T, handler http.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/rpc", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func decodeObject(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(payload, &decoded))
	return decoded
}
