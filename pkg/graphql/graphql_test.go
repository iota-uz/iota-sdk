package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	coregraph "github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	warehousegraph "github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
)

type fakeExecutableSchema struct{}

func (fakeExecutableSchema) Schema() *ast.Schema {
	return &ast.Schema{}
}

func (fakeExecutableSchema) Complexity(string, string, int, map[string]any) (int, bool) {
	return 0, false
}

func (fakeExecutableSchema) Exec(context.Context) gqlgraphql.ResponseHandler {
	return func(context.Context) *gqlgraphql.Response {
		return &gqlgraphql.Response{}
	}
}

func TestAddExecutor_RegistersIntrospectionForEachExecutor(t *testing.T) {
	originalRegisterIntrospection := registerIntrospection
	defer func() {
		registerIntrospection = originalRegisterIntrospection
	}()

	first := executor.New(fakeExecutableSchema{})
	second := executor.New(fakeExecutableSchema{})
	third := executor.New(fakeExecutableSchema{})

	testCases := []struct {
		name               string
		input              []*executor.Executor
		expectedExecutors  []*executor.Executor
		expectedRegistered []*executor.Executor
	}{
		{
			name:               "no executors",
			input:              nil,
			expectedExecutors:  nil,
			expectedRegistered: nil,
		},
		{
			name:               "single executor",
			input:              []*executor.Executor{first},
			expectedExecutors:  []*executor.Executor{first},
			expectedRegistered: []*executor.Executor{first},
		},
		{
			name:               "multiple executors",
			input:              []*executor.Executor{first, second, third},
			expectedExecutors:  []*executor.Executor{first, second, third},
			expectedRegistered: []*executor.Executor{first, second, third},
		},
		{
			name:               "nil entries are skipped",
			input:              []*executor.Executor{first, nil, second, nil},
			expectedExecutors:  []*executor.Executor{first, second},
			expectedRegistered: []*executor.Executor{first, second},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var calledWith []*executor.Executor
			registerIntrospection = func(server *Handler, exec *executor.Executor) {
				calledWith = append(calledWith, exec)
			}

			handler := &Handler{}
			handler.AddExecutor(tc.input...)

			require.Equal(t, tc.expectedExecutors, handler.execs)
			require.Equal(t, tc.expectedRegistered, calledWith)
		})
	}
}

func TestMergedIntrospectionIncludesTypesFromAllExecutors(t *testing.T) {
	ctx := context.Background()
	originalRegisterIntrospection := registerIntrospection
	defer func() {
		registerIntrospection = originalRegisterIntrospection
	}()

	registerIntrospection = func(server *Handler, exec *executor.Executor) {
		server.Use(map[*executor.Executor]gqlgraphql.HandlerExtension{
			exec: extension.Introspection{},
		})
	}

	handler := NewBaseServer(coregraph.NewExecutableSchema(coregraph.Config{}), nil)
	handler.AddExecutor(executor.New(warehousegraph.NewExecutableSchema(warehousegraph.Config{})))

	requestBody := map[string]any{
		"query": "query IntrospectionQuery { __schema { types { name } } }",
	}
	payload, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &parsed))

	data, ok := parsed["data"].(map[string]any)
	require.True(t, ok)

	schema, ok := data["__schema"].(map[string]any)
	require.True(t, ok)

	types, ok := schema["types"].([]any)
	require.True(t, ok)

	typeNames := make(map[string]struct{}, len(types))
	for _, value := range types {
		obj, ok := value.(map[string]any)
		if !ok {
			continue
		}

		name, ok := obj["name"].(string)
		if !ok || name == "" {
			continue
		}
		typeNames[name] = struct{}{}
	}

	_, hasCoreType := typeNames["User"]
	_, hasWarehouseType := typeNames["InventoryPosition"]
	require.True(t, hasCoreType)
	require.True(t, hasWarehouseType)
}

func TestSameOriginValidator(t *testing.T) {
	ctx := context.Background()
	httpCfg := &httpconfig.Config{
		Port:           3200,
		Domain:         "erp.example.com",
		OriginOverride: "https://erp.example.com",
		AllowedOrigins: []string{"https://partner.example.org"},
	}
	appCfg := &appconfig.Config{Environment: "production"}
	validator := SameOriginValidator(httpCfg, appCfg)

	testCases := []struct {
		name   string
		origin string
		want   bool
	}{
		{name: "no origin header", origin: "", want: true},
		{name: "configured origin", origin: "https://erp.example.com", want: true},
		{name: "configured allowed origin", origin: "https://partner.example.org", want: true},
		{name: "origin with path and query is normalized", origin: "https://erp.example.com/some/path?q=1", want: true},
		{name: "foreign origin", origin: "https://evil.example.com", want: false},
		{name: "localhost rejected in production", origin: "http://localhost:3200", want: false},
		{name: "malformed origin", origin: "::::", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", nil)
			if tc.origin != "" {
				r.Header.Set("Origin", tc.origin)
			}
			require.Equal(t, tc.want, validator(r))
		})
	}

	t.Run("allows localhost in development", func(t *testing.T) {
		devCfg := &appconfig.Config{Environment: "development"}
		devValidator := SameOriginValidator(httpCfg, devCfg)

		for _, origin := range []string{
			"http://localhost:3200",
			"http://127.0.0.1:3200",
			"http://[::1]:3200",
			"https://localhost:3200",
		} {
			r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", nil)
			r.Header.Set("Origin", origin)
			require.True(t, devValidator(r), origin)
		}

		r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", nil)
		r.Header.Set("Origin", "http://localhost:9999")
		require.False(t, devValidator(r))
	})
}

func TestMyPOST_RejectsDisallowedOrigin(t *testing.T) {
	ctx := context.Background()
	handler := NewBaseServer(
		coregraph.NewExecutableSchema(coregraph.Config{}),
		nil,
		WithOriginValidator(func(r *http.Request) bool {
			return false
		}),
	)

	r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", bytes.NewBufferString(`{"query":"{ id }"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Header().Get("Content-Type"), "application/json")

	var body struct {
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Errors, 1)
	require.Equal(t, "origin not allowed", body.Errors[0].Message)
}
