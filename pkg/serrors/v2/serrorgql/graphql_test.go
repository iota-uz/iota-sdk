package serrorgql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	sdkgraphql "github.com/iota-uz/iota-sdk/pkg/graphql"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorgql"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type failingSchema struct{ err error }

func (failingSchema) Schema() *ast.Schema {
	return gqlparser.MustLoadSchema(&ast.Source{Input: "type Query { account: String }"})
}
func (failingSchema) Complexity(string, string, int, map[string]any) (int, bool) { return 0, false }
func (s failingSchema) Exec(ctx context.Context) gql.ResponseHandler {
	return func(ctx context.Context) *gql.Response {
		gql.AddError(ctx, &gqlerror.Error{Err: s.err, Message: s.err.Error(), Path: ast.Path{ast.PathName("account")}, Locations: []gqlerror.Location{{Line: 1, Column: 3}}})
		return &gql.Response{Data: json.RawMessage(`{"account":null}`)}
	}
}
func TestSDKHandlerPresenterExecutionAndProtocol(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		err         error
		status      int
		code        string
	}{{"execution internal", "{ account }", serrors.NewInternal("secret SQL").WithCause(errors.New("password=secret")), 200, "internal"}, {"execution unauthorized", "{ account }", serrors.NewUnauthenticated("secret"), 200, "UNAUTHORIZED"}, {"protocol invalid field", "{ missing }", serrors.NewInternal("secret"), 422, "GRAPHQL_VALIDATION_FAILED"}, {"protocol parse", "{", serrors.NewInternal("secret"), 422, "GRAPHQL_PARSE_FAILED"}} {
		t.Run(tc.name, func(t *testing.T) {
			ex := executor.New(failingSchema{err: tc.err})
			handler := sdkgraphql.NewHandler(ex, nil)
			handler.SetErrorPresenter(map[*executor.Executor]gql.ErrorPresenterFunc{ex: serrorgql.Presenter(nil)})
			body, _ := json.Marshal(map[string]string{"query": tc.query})
			r := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(string(body)))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			var response struct {
				Errors []*gqlerror.Error `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
			require.NotEmpty(t, response.Errors)
			require.Equal(t, tc.code, response.Errors[0].Extensions["code"])
			require.NotContains(t, w.Body.String(), "secret")
			if tc.status == 200 {
				require.Equal(t, ast.Path{ast.PathName("account")}, response.Errors[0].Path)
				require.Equal(t, []gqlerror.Location{{Line: 1, Column: 3}}, response.Errors[0].Locations)
			}
		})
	}
}

type declaredError struct{ error }

func (e declaredError) Unwrap() error { return e.error }

func (declaredError) GraphQLCode() string { return "VEHICLE_OWNER_INN_REQUIRED" }
func (declaredError) GraphQLExtensions() map[string]serrors.Value {
	return map[string]serrors.Value{"owner": serrors.Text("Synthetic company"), "code": serrors.Text("override"), "reason": serrors.Text("override")}
}
func TestPresenterExplicitCarrierAndInternalOverride(t *testing.T) {
	carrier := declaredError{serrors.NewInvalid("private SQL").WithReason("owner_inn_missing")}
	for _, tc := range []struct {
		err   error
		code  string
		owner bool
	}{{serrors.Wrap("lookup", carrier), "VEHICLE_OWNER_INN_REQUIRED", true}, {serrors.NewInternal("private failure").WithCause(carrier), "internal", false},
		{errors.Join(serrors.NewInternal("first"), carrier), "internal", false},
		{errors.Join(context.Canceled, carrier), "canceled", false},
		{errors.Join(errors.New("unknown"), carrier), "VEHICLE_OWNER_INN_REQUIRED", true}} {
		p := serrorgql.Presenter(nil)(context.Background(), tc.err)
		require.Equal(t, tc.code, p.Extensions["code"])
		require.NotContains(t, p.Message, "private")
		if tc.owner {
			require.Equal(t, "Synthetic company", p.Extensions["owner"])
			require.Equal(t, serrors.Reason("owner_inn_missing"), p.Extensions["reason"])
		} else {
			require.NotContains(t, p.Extensions, "owner")
			require.NotContains(t, p.Extensions, "reason")
		}
	}
}
func TestPresenterLogsOneBoundedExecutionEvent(t *testing.T) {
	var output bytes.Buffer
	logger := logrus.New()
	logger.SetOutput(&output)
	logger.SetFormatter(&logrus.JSONFormatter{})
	ctx := context.WithValue(context.Background(), constants.LoggerKey, logger.WithField("request-id", "synthetic-graphql-request"))
	err := serrors.NewInvalid("").WithOp("synthetic.graphql.op").WithReason("synthetic_reason").WithCause(errors.New("private SQL cause"))
	result := serrorgql.Presenter(nil)(ctx, err)
	require.Equal(t, "invalid", result.Extensions["code"])
	require.Equal(t, 1, strings.Count(output.String(), "GraphQL request failed"))
	require.Contains(t, output.String(), `"error.op":"synthetic.graphql.op"`)
	require.Contains(t, output.String(), `"error.reason":"synthetic_reason"`)
	require.Contains(t, output.String(), `"request_id":"synthetic-graphql-request"`)
	require.NotContains(t, output.String(), "private SQL cause")
	output.Reset()
	protocol := serrorgql.Presenter(nil)(ctx, &gqlerror.Error{Message: "parse", Extensions: map[string]any{"code": "GRAPHQL_PARSE_FAILED"}})
	require.Equal(t, "GRAPHQL_PARSE_FAILED", protocol.Extensions["code"])
	require.Empty(t, output.String())
}
