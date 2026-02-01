package tools

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
)

// mockAssistantsExecutor is a mock implementation for testing
type mockAssistantsExecutor struct {
	outputs []types.CodeInterpreterOutput
	err     error
}

func (m *mockAssistantsExecutor) ExecuteCodeInterpreter(ctx context.Context, messageID uuid.UUID, userMessage string) ([]types.CodeInterpreterOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.outputs, nil
}

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

		// Check required fields
		props, ok := params["properties"].(map[string]any)
		assert.True(t, ok)
		assert.Contains(t, props, "description")
		assert.Contains(t, props, "code")

		required, ok := params["required"].([]string)
		assert.True(t, ok)
		assert.Contains(t, required, "description")
		assert.Contains(t, required, "code")
	})

	t.Run("Call_WithoutExecutor_ReturnsError", func(t *testing.T) {
		tool := NewCodeInterpreterTool()
		ctx := context.Background()
		input := `{"description": "Print hello", "code": "print('hello')"}`

		result, err := tool.Call(ctx, input)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "executor not configured")
	})

	t.Run("Call_WithExecutor_Success", func(t *testing.T) {
		// Setup mock executor with outputs
		mockOutputs := []types.CodeInterpreterOutput{
			{
				ID:        uuid.New(),
				MessageID: uuid.New(),
				Name:      "chart.png",
				MimeType:  "image/png",
				URL:       "https://example.com/chart.png",
				Size:      1024,
			},
		}
		executor := &mockAssistantsExecutor{outputs: mockOutputs}

		tool := NewCodeInterpreterTool(WithAssistantsExecutor(executor))
		ctx := context.Background()
		input := `{"description": "Generate chart", "code": "import matplotlib.pyplot as plt\nplt.plot([1,2,3])\nplt.savefig('chart.png')"}`

		result, err := tool.Call(ctx, input)

		assert.NoError(t, err)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "completed")
		assert.Contains(t, result, "chart.png")
	})

	t.Run("Call_WithExecutor_ExecutionError", func(t *testing.T) {
		// Setup mock executor that returns an error
		executor := &mockAssistantsExecutor{err: assert.AnError}

		tool := NewCodeInterpreterTool(WithAssistantsExecutor(executor))
		ctx := context.Background()
		input := `{"description": "Failing code", "code": "raise Exception('test error')"}`

		result, err := tool.Call(ctx, input)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "code execution failed")
	})

	t.Run("Call_InvalidJSON_ReturnsError", func(t *testing.T) {
		executor := &mockAssistantsExecutor{}
		tool := NewCodeInterpreterTool(WithAssistantsExecutor(executor))
		ctx := context.Background()
		input := `invalid json`

		result, err := tool.Call(ctx, input)

		assert.Error(t, err)
		assert.Empty(t, result)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}
