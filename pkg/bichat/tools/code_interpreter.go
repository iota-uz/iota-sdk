package tools

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// CodeInterpreterTool is a marker tool that tells the Model layer to enable
// native code execution via the OpenAI Responses API.
// The API runs Python code internally and returns outputs (logs, images).
// Call() should never be invoked — if it is, it returns an error.
type CodeInterpreterTool struct{}

// NewCodeInterpreterTool creates a new code interpreter marker tool.
func NewCodeInterpreterTool() agents.Tool {
	return &CodeInterpreterTool{}
}

func (t *CodeInterpreterTool) Name() string { return "code_interpreter" }

func (t *CodeInterpreterTool) Description() string {
	return "Execute Python code for data analysis, calculations, and visualization. " +
		"Supports pandas, numpy, matplotlib, seaborn for data processing and charting. " +
		"Can generate images (PNG), CSVs, and other file outputs."
}

func (t *CodeInterpreterTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Description of what the code does",
			},
			"code": map[string]any{
				"type":        "string",
				"description": "Python code to execute",
			},
		},
		"required": []string{"description", "code"},
	}
}

// Call returns an error because code interpreter is handled natively by the API.
func (t *CodeInterpreterTool) Call(_ context.Context, _ string) (string, error) {
	return FormatToolError(
		ErrCodeServiceUnavailable,
		"code_interpreter is handled natively by the API",
		"This tool should not be called directly; it is executed by the model provider",
		"This is a marker tool — no action needed",
	), nil
}
