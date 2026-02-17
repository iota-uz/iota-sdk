package graphql

import (
	"context"
	"testing"

	gqlgraphql "github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
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
