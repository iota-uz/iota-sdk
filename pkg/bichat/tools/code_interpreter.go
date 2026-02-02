package tools

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// CodeInterpreterTool is a tool that executes Python code via OpenAI's Assistants API.
//
// Architecture:
// - When used with OpenAI's Assistants API: Code execution happens in OpenAI's sandboxed environment
// - Supports pandas, numpy, matplotlib, seaborn for data processing and visualization
// - Generates file outputs (images, CSVs, etc.) which are downloaded and stored
//
// The tool requires an AssistantsExecutor to handle the actual code execution.
// If no executor is provided, the tool acts as a placeholder that returns an error.
type CodeInterpreterTool struct {
	executor AssistantsExecutor // Optional: Executes code via Assistants API
}

// AssistantsExecutor is an interface for executing code via OpenAI Assistants API.
// This allows us to inject the execution logic without creating circular dependencies.
type AssistantsExecutor interface {
	// ExecuteCodeInterpreter executes Python code and returns file outputs
	ExecuteCodeInterpreter(ctx context.Context, messageID uuid.UUID, userMessage string) ([]types.CodeInterpreterOutput, error)
}

// CodeInterpreterToolOption is a functional option for configuring CodeInterpreterTool.
type CodeInterpreterToolOption func(*CodeInterpreterTool)

// WithAssistantsExecutor sets the executor for code execution via Assistants API.
func WithAssistantsExecutor(executor AssistantsExecutor) CodeInterpreterToolOption {
	return func(t *CodeInterpreterTool) {
		t.executor = executor
	}
}

// NewCodeInterpreterTool creates a new code interpreter tool.
//
// Without an executor, the tool acts as a placeholder.
// With an executor, it can execute code via OpenAI Assistants API.
//
// Example:
//
//	// Placeholder (no execution)
//	tool := tools.NewCodeInterpreterTool()
//
//	// With Assistants API execution
//	executor := llmproviders.NewAssistantsClient(openaiClient, fileStorage)
//	tool := tools.NewCodeInterpreterTool(tools.WithAssistantsExecutor(executor))
func NewCodeInterpreterTool(opts ...CodeInterpreterToolOption) agents.Tool {
	tool := &CodeInterpreterTool{}
	for _, opt := range opts {
		opt(tool)
	}
	return tool
}

// Name returns the tool name recognized by OpenAI.
func (t *CodeInterpreterTool) Name() string {
	return "code_interpreter"
}

// Description returns the tool description for the LLM.
func (t *CodeInterpreterTool) Description() string {
	return "Execute Python code for data analysis, calculations, and visualization. " +
		"Supports pandas, numpy, matplotlib, seaborn for data processing and charting. " +
		"Can generate images (PNG), CSVs, and other file outputs."
}

// Parameters returns the schema for code execution requests.
// The LLM provides the code to execute and a description of what it does.
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

// Call executes Python code via the Assistants API executor.
// Returns an error if no executor is configured.
func (t *CodeInterpreterTool) Call(ctx context.Context, input string) (string, error) {
	const op serrors.Op = "CodeInterpreterTool.Call"

	// Check if executor is configured
	if t.executor == nil {
		return "", serrors.E(op, "code interpreter executor not configured")
	}

	// Parse input to extract code and description
	var req struct {
		Description string `json:"description"`
		Code        string `json:"code"`
	}
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		return "", serrors.E(op, err, "failed to parse code interpreter request")
	}

	// Generate a message ID for associating outputs
	// Note: This is a temporary ID - the executor/service layer should replace it with the real message ID
	messageID := uuid.New()

	// Execute code via Assistants API
	// The userMessage should contain the code and context
	userMessage := req.Description + "\n\n```python\n" + req.Code + "\n```"
	outputs, err := t.executor.ExecuteCodeInterpreter(ctx, messageID, userMessage)
	if err != nil {
		return "", serrors.E(op, err, "code execution failed")
	}

	// Build result with execution status and file outputs
	result := map[string]any{
		"status":  "completed",
		"message": "Code executed successfully",
		"outputs": outputs,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", serrors.E(op, err, "failed to marshal result")
	}

	return string(resultJSON), nil
}
