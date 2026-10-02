package serrorrpc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/pkg/appletengine/rpc"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorrpc"
	"github.com/stretchr/testify/require"
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
			require.EqualValues(t, -32601, response.Error.Code)
		})
	}
}
