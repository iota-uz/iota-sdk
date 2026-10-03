package serrorgql_test

import (
	"context"
	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2"
	"github.com/iota-uz/iota-sdk/pkg/serrors/v2/serrorgql"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"testing"
)

func TestProtocolExtensionCannotBypassExecutionClassification(t *testing.T) {
	protocol := &gqlerror.Error{Message: "private SQL secret", Extensions: map[string]any{"code": "GRAPHQL_PARSE_FAILED", "private": "secret"}}
	for _, tc := range []struct {
		name string
		ctx  context.Context
		err  error
	}{
		{"explicit outer classification", context.Background(), serrors.NewInternal("private context").WithCause(protocol)},
		{"execution error", graphql.WithOperationContext(context.Background(), &graphql.OperationContext{Operation: &ast.OperationDefinition{Operation: ast.Query}}), protocol},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := serrorgql.Presenter(nil)(tc.ctx, tc.err)
			require.Equal(t, "internal", result.Extensions["code"])
			require.NotContains(t, result.Message, "secret")
			require.NotContains(t, result.Extensions, "private")
		})
	}
}
