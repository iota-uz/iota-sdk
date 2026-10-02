package serrorgql_test

import (
	"context"
	"encoding/json"
	"errors"
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
