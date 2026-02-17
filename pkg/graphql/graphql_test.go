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
			calledWith := make([]*executor.Executor, 0)
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
	originalRegisterIntrospection := registerIntrospection
	defer func() {
		registerIntrospection = originalRegisterIntrospection
	}()

	registerIntrospection = func(server *Handler, exec *executor.Executor) {
		server.Use(map[*executor.Executor]gqlgraphql.HandlerExtension{
			exec: extension.Introspection{},
		})
	}

	handler := NewBaseServer(coregraph.NewExecutableSchema(coregraph.Config{}))
	handler.AddExecutor(executor.New(warehousegraph.NewExecutableSchema(warehousegraph.Config{})))

	requestBody := map[string]any{
		"query": "query IntrospectionQuery { __schema { types { name } } }",
	}
	payload, err := json.Marshal(requestBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewReader(payload))
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
