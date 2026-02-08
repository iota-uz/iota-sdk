package agents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToolSchema_GeneratesObjectSchemaWithRequiredAndTags(t *testing.T) {
	t.Parallel()

	type input struct {
		Query string `json:"query" jsonschema:"description=The search query"`
		Limit int    `json:"limit,omitempty" jsonschema:"description=Max results;default=5;minimum=1;maximum=20"`
	}

	s := ToolSchema[input]()
	require.Equal(t, "object", s["type"])

	props, ok := s["properties"].(map[string]any)
	require.True(t, ok)

	query, ok := props["query"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "string", query["type"])
	require.Equal(t, "The search query", query["description"])

	limit, ok := props["limit"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "integer", limit["type"])
	// Numbers are represented as float64 after clone/unmarshal.
	require.InEpsilon(t, float64(5), limit["default"], 1e-9)
	require.InEpsilon(t, float64(1), limit["minimum"], 1e-9)
	require.InEpsilon(t, float64(20), limit["maximum"], 1e-9)

	required, ok := s["required"].([]any)
	require.True(t, ok)
	require.Contains(t, required, "query")
	require.NotContains(t, required, "limit")
}

func TestTypedTool_ParseErrorHandler(t *testing.T) {
	t.Parallel()

	type input struct {
		Query string `json:"query"`
	}

	tool := NewTypedTool[input](
		"test_tool",
		"test",
		func(ctx context.Context, in input) (string, error) {
			return in.Query, nil
		},
		WithTypedToolParseErrorHandler[input](func(ctx context.Context, raw string, err error) (string, error) {
			require.Error(t, err)
			return "handled", nil
		}),
	)

	out, err := tool.Call(context.Background(), "{invalid json")
	require.NoError(t, err)
	require.Equal(t, "handled", out)

	out, err = tool.Call(context.Background(), `{"query":"ok"}`)
	require.NoError(t, err)
	require.Equal(t, "ok", out)

	out, err = tool.Call(context.Background(), `{"query":"ok","extra":true}`)
	// Extra fields are accepted by json.Unmarshal.
	require.NoError(t, err)
	require.Equal(t, "ok", out)
}
