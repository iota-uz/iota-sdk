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

func TestHandlerAddExecutorRegistersIntrospectionForEachExecutor(t *testing.T) {
	originalRegisterIntrospection := registerIntrospection
	defer func() {
		registerIntrospection = originalRegisterIntrospection
	}()

	calledWith := make([]*executor.Executor, 0)
	registerIntrospection = func(server *Handler, exec *executor.Executor) {
		calledWith = append(calledWith, exec)
	}

	handler := &Handler{}
	first := executor.New(fakeExecutableSchema{})
	second := executor.New(fakeExecutableSchema{})

	handler.AddExecutor(first, nil, second)

	require.Equal(t, []*executor.Executor{first, second}, handler.execs)
	require.Equal(t, []*executor.Executor{first, second}, calledWith)
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
