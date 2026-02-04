package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebSearchTool_Name(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool()
	assert.Equal(t, "web_search", tool.Name())
}

func TestWebSearchTool_Description(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool()
	desc := tool.Description()
	assert.NotEmpty(t, desc)
	assert.Contains(t, desc, "Search")
	assert.Contains(t, desc, "web")
}

func TestWebSearchTool_Parameters(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool()
	params := tool.Parameters()

	assert.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	properties, ok := params["properties"].(map[string]any)
	assert.True(t, ok)
	assert.Contains(t, properties, "query")

	required, ok := params["required"].([]string)
	assert.True(t, ok)
	assert.Contains(t, required, "query")
}

func TestWebSearchTool_Call(t *testing.T) {
	t.Parallel()

	tool := NewWebSearchTool()
	ctx := context.Background()

	input := `{"query": "test search"}`

	// Tool is enabled but not yet implemented - should return formatted error
	// This allows LLM to gracefully handle unavailable search
	result, err := tool.Call(ctx, input)

	// Should return formatted error with helpful message
	assert.NotEmpty(t, result)
	require.Error(t, err)
	assert.Contains(t, result, "SERVICE_UNAVAILABLE")
	assert.Contains(t, result, "not yet implemented")
	assert.Contains(t, err.Error(), "web_search implementation pending")
}
