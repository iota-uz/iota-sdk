package serrorrpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorrpc"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectApplicationCodes(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{{serrors.NewInvalid("secret"), "invalid"}, {serrors.NewNotFound("secret"), "not_found"}, {serrors.NewPermissionDenied("secret"), "forbidden"}, {serrors.NewInternal("secret").WithCause(errors.New("SQL secret")), "internal"}, {serrors.NewInvalid("secret").WithFields(serrors.FieldViolation{Field: "Email", Reason: "required"}), "validation"}} {
		p := serrorrpc.Project(tc.err, nil)
		require.Equal(t, tc.code, p.Code)
		require.NotContains(t, p.Message, "secret")
		if tc.code == "validation" {
			require.Equal(t, []serrors.PublicField{{Field: "Email", Reason: "required", Message: "Check the supplied information."}}, p.Details["fields"])
		} else {
			require.Nil(t, p.Details)
		}
	}
}

type explicitCarrierError struct{ error }

func (explicitCarrierError) RPCCode() any       { return "specific" }
func (explicitCarrierError) RPCMessage() string { return "Approved message" }
func (explicitCarrierError) RPCDetails() any    { return map[string]any{"retry": false} }

func TestSDKRPCBoundaryPreservesCarrierSentinelAndProtocolPriority(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code any
	}{{"carrier", explicitCarrierError{serrors.NewInvalid("secret")}, "specific"}, {"sentinel", serrors.NewNotFound("secret").WithCause(applets.ErrPermissionDenied), "forbidden"}, {"classification", serrors.NewNotFound("secret"), "not_found"}} {
		t.Run(tc.name, func(t *testing.T) {
			registry := rpc.NewRegistry()
			require.NoError(t, registry.RegisterPublicContract("test", "test.failure", applets.RPCMethod{Handler: func(context.Context, json.RawMessage) (any, error) { return nil, tc.err }}, nil, rpc.Query(false, 0), rpc.Public()))
			d := rpc.NewDispatcher(registry, nil, nil)
			r := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"test.failure","params":{}}`))
			w := httptest.NewRecorder()
			d.HandlePublicHTTP(w, r)
			require.Equal(t, 200, w.Code)
			var response struct {
				Error struct {
					Code    any            `json:"code"`
					Message string         `json:"message"`
					Details map[string]any `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.Equal(t, tc.code, response.Error.Code)
			require.NotContains(t, w.Body.String(), "secret")
			if tc.name == "carrier" {
				require.Equal(t, "Approved message", response.Error.Message)
				require.Equal(t, map[string]any{"retry": false}, response.Error.Details)
			}
			protocol := httptest.NewRecorder()
			d.HandlePublicHTTP(protocol, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"2","method":"test.missing"}`)))
			require.NoError(t, json.Unmarshal(protocol.Body.Bytes(), &response))
			require.InDelta(t, -32601.0, response.Error.Code, 0.0)
		})
	}
}

func TestSDKRPCBoundaryProjectsSafeValidationFields(t *testing.T) {
	registry := rpc.NewRegistry()
	err := serrors.NewInvalid("private SQL").WithFields(serrors.FieldViolation{Field: "Email", Reason: "required"})
	require.NoError(t, registry.RegisterPublicContract("test", "test.validation", applets.RPCMethod{Handler: func(context.Context, json.RawMessage) (any, error) { return nil, err }}, nil, rpc.Query(false, 0), rpc.Public()))
	w := httptest.NewRecorder()
	rpc.NewDispatcher(registry, nil, nil).HandlePublicHTTP(w, httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"test.validation","params":{}}`)))
	require.Equal(t, 200, w.Code)
	require.JSONEq(t, `{"id":"1","jsonrpc":"2.0","error":{"code":"validation","message":"Check the supplied information.","details":{"fields":[{"field":"Email","reason":"required","message":"Check the supplied information."}]}}}`, w.Body.String())
}

func TestSDKRPCBoundaryLogsBoundedContextWithoutDiagnostics(t *testing.T) {
	registry := rpc.NewRegistry()
	err := serrors.NewInvalid("password=secret").WithOp("account.Update").WithReason("required").WithMeta(map[string]serrors.Value{"token": serrors.Text("secret")})
	require.NoError(t, registry.RegisterPublicContract("test", "test.logs", applets.RPCMethod{Handler: func(context.Context, json.RawMessage) (any, error) { return nil, err }}, nil, rpc.Query(false, 0), rpc.Public()))
	var logs bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&logs)
	logger.SetFormatter(&logrus.JSONFormatter{})
	r := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"id":"1","method":"test.logs","params":{}}`))
	r.Header.Set("X-Iota-Request-Id", "correlation-123")
	rpc.NewDispatcher(registry, nil, logger).HandlePublicHTTP(httptest.NewRecorder(), r)
	var entry map[string]any
	require.NoError(t, json.NewDecoder(&logs).Decode(&entry))
	require.Equal(t, "info", entry["level"])
	require.Equal(t, "invalid", entry["error.code"])
	require.Equal(t, "account.Update", entry["error.op"])
	require.Equal(t, "required", entry["error.reason"])
	require.Equal(t, "correlation-123", entry["request_id"])
	require.NotContains(t, logs.String(), "secret")
	require.NotContains(t, entry, "error")
}
