package utility

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodeInterpreterTool(t *testing.T) {
	t.Parallel()

	t.Run("Name", func(t *testing.T) {
		tool := NewCodeInterpreterTool()
		assert.Equal(t, "code_interpreter", tool.Name())
	})

	t.Run("Description", func(t *testing.T) {
		tool := NewCodeInterpreterTool()
		desc := tool.Description()
		assert.NotEmpty(t, desc)
		assert.Contains(t, desc, "Python")
		assert.Contains(t, desc, "data analysis")
	})

	t.Run("Parameters", func(t *testing.T) {
		tool := NewCodeInterpreterTool()
		params := tool.Parameters()
		assert.NotNil(t, params)
		assert.Equal(t, "object", params["type"])

		props, ok := params["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "description")
		assert.Contains(t, props, "code")

		required, ok := params["required"].([]string)
		assert.True(t, ok)
		assert.Contains(t, required, "description")
		assert.Contains(t, required, "code")
	})

	t.Run("Call_ReturnsMarkerError", func(t *testing.T) {
		tool := NewCodeInterpreterTool()
		ctx := context.Background()
		input := `{"description": "Print hello", "code": "print('hello')"}`

		result, err := tool.Call(ctx, input)

		// Marker tool — returns formatted error string, nil Go error
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "SERVICE_UNAVAILABLE")
		assert.Contains(t, result, "natively")
	})
}
