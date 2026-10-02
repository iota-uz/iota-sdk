package serrorgql_test

import (
	"context"
	"encoding/json"
	"errors"
	gql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/iota-uz/go-i18n/v2/i18n"
	sdkgraphql "github.com/iota-uz/iota-sdk/pkg/graphql"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	serrors "github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorgql"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"golang.org/x/text/language"
	"net/http/httptest"
	"strings"
	"testing"
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
			if tc.status != 200 {
				handler.SetErrorPresenter(map[*executor.Executor]gql.ErrorPresenterFunc{ex: serrorgql.Presenter(nil)})
			}
			body, _ := json.Marshal(map[string]string{"query": tc.query})
			r := httptest.NewRequest("POST", "/query", strings.NewReader(string(body)))
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

func TestSDKHandlerDefaultPresenterUsesRequestLocaleAndSafeFallback(t *testing.T) {
	for _, translation := range []string{"Request denied.", " "} {
		t.Run(translation, func(t *testing.T) {
			bundle := i18n.NewBundle(language.English)
			bundle.AddMessages(language.English, &i18n.Message{ID: "Public.denied", Other: translation})
			ex := executor.New(failingSchema{err: serrors.NewPermissionDenied("private credential").WithPublic(serrors.Message{ID: "Public.denied"})})
			handler := sdkgraphql.NewHandler(ex, nil)
			r := httptest.NewRequest("POST", "/query", strings.NewReader(`{"query":"{ account }"}`))
			r.Header.Set("Content-Type", "application/json")
			r = r.WithContext(intl.WithLocalizer(r.Context(), i18n.NewLocalizer(bundle, "en")))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			require.Equal(t, 200, w.Code)
			require.NotContains(t, w.Body.String(), "private")
			expected := translation
			if strings.TrimSpace(translation) == "" {
				expected = "You do not have permission to perform this operation."
			}
			require.Contains(t, w.Body.String(), expected)
		})
	}
}
