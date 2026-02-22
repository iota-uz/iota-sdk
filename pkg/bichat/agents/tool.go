package agents

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ErrStructuredToolOutput is returned by StructuredTool when the tool returns
// a structured error payload (e.g. validation failure) that should be presented
// to the LLM as the tool output without propagating a Go error. Callers should
// format the result and return (formatted, nil) when errors.Is(err, ErrStructuredToolOutput).
var ErrStructuredToolOutput = errors.New("structured tool error output")

// BuiltinTool constants for special tools handled by the executor.
// These tools have special semantics that affect the ReAct loop flow.
const (
	// ToolAskUserQuestion pauses execution and waits for user input.
	// When called, the executor saves a checkpoint and yields an interrupt event.
	// The Result.Interrupt field will contain the interrupt details.
	ToolAskUserQuestion = "ask_user_question"

	// ToolFinalAnswer terminates the ReAct loop and returns the result.
	// This is the standard way for an agent to complete its task.
	ToolFinalAnswer = "final_answer"

	// ToolTask delegates work to a sub-agent.
	// The executor handles spawning and managing the child agent.
	ToolTask = "task"
)

// Tool defines the contract for agent tools.
// Tools use simple string I/O pattern (LangChainGo style) for simplicity:
// - Input is a JSON string containing tool parameters
// - Output is a string result that will be sent back to the LLM
//
// Tools are executed during the ReAct loop when the LLM decides to call them.
// The executor manages tool lookup, execution, and result formatting.
type Tool interface {
	// Name returns the unique tool identifier.
	// This must match what the LLM uses to invoke the tool.
	Name() string

	// Description returns a human-readable description for the LLM.
	// This is included in the system prompt to help the LLM understand
	// when and how to use the tool.
	Description() string

	// Parameters returns the JSON Schema for tool parameters.
	// This is used to generate provider tool definitions and validate input.
	// Example:
	//   map[string]any{
	//     "type": "object",
	//     "properties": map[string]any{
	//       "query": map[string]any{
	//         "type": "string",
	//         "description": "The search query",
	//       },
	//     },
	//     "required": []string{"query"},
	//   }
	Parameters() map[string]any

	// Call executes the tool with the given input (JSON string).
	// Returns the result as a string (will be sent back to LLM).
	// The context may contain tenant ID, user info, and other request-scoped data.
	Call(ctx context.Context, input string) (string, error)
}

// ToolConcurrency is an optional interface that tools can implement to influence
// concurrent execution behavior when multiple tool calls are returned in one turn.
//
// Tools with the same non-empty ConcurrencyKey will be executed serially
// (while tools with different keys may execute in parallel).
type ToolConcurrency interface {
	ConcurrencyKey() string
}

// StructuredTool extends Tool with structured output support.
// Tools that implement this interface return typed payloads instead of
// pre-formatted strings. The executor uses a FormatterRegistry to
// convert the payload to a string before sending it to the LLM.
//
// For backward compatibility, tools may implement both Tool and StructuredTool.
// The executor will prefer CallStructured when available.
type StructuredTool interface {
	Tool

	// CallStructured executes the tool and returns a structured result.
	// The returned ToolResult contains a codec ID and typed payload.
	// If the tool encounters a handled error (e.g., validation failure),
	// it should return a ToolResult with a "tool-error" codec ID and
	// ToolErrorPayload, not a Go error.
	CallStructured(ctx context.Context, input string) (*types.ToolResult, error)
}

// StreamingTool extends Tool with the ability to emit events during execution.
// Tools that implement this interface can push intermediate events (e.g., child
// tool calls, thinking content) to the parent executor's event stream.
//
// The executor will prefer CallStreaming over Call when available.
// Tools that do not need event emission should implement only the Tool interface.
type StreamingTool interface {
	Tool
	// CallStreaming executes the tool and pushes intermediate events via emit.
	// The emit callback returns false if the consumer has stopped listening.
	CallStreaming(ctx context.Context, input string, emit EventEmitter) (string, error)
}

// ToolFunc is a convenience type for creating simple tools from functions.
// It implements the Tool interface using struct fields instead of methods.
//
// Example:
//
//	tool := &ToolFunc{
//	    ToolName:        "echo",
//	    ToolDescription: "Echoes the input back",
//	    ToolParameters:  map[string]any{"type": "object", "properties": map[string]any{}},
//	    Fn: func(ctx context.Context, input string) (string, error) {
//	        return input, nil
//	    },
//	}
type ToolFunc struct {
	ToolName        string
	ToolDescription string
	ToolParameters  map[string]any
	Fn              func(ctx context.Context, input string) (string, error)
}

// Name returns the unique tool identifier.
func (t *ToolFunc) Name() string { return t.ToolName }

// Description returns a human-readable description for the LLM.
func (t *ToolFunc) Description() string { return t.ToolDescription }

// Parameters returns the JSON Schema for tool parameters.
func (t *ToolFunc) Parameters() map[string]any { return t.ToolParameters }

// Call executes the tool with the given input.
func (t *ToolFunc) Call(ctx context.Context, input string) (string, error) {
	return t.Fn(ctx, input)
}

// NewTool creates a simple tool from a function.
// This is a convenience constructor for creating tools without defining a struct.
//
// Example:
//
//	tool := NewTool(
//	    "greet",
//	    "Greets a person by name",
//	    map[string]any{
//	        "type": "object",
//	        "properties": map[string]any{
//	            "name": map[string]any{"type": "string"},
//	        },
//	        "required": []string{"name"},
//	    },
//	    func(ctx context.Context, input string) (string, error) {
//	        var params struct{ Name string }
//	        if err := json.Unmarshal([]byte(input), &params); err != nil {
//	            return "", err
//	        }
//	        return fmt.Sprintf("Hello, %s!", params.Name), nil
//	    },
//	)
func NewTool(
	name, description string,
	parameters map[string]any,
	fn func(ctx context.Context, input string) (string, error),
) Tool {
	return &ToolFunc{
		ToolName:        name,
		ToolDescription: description,
		ToolParameters:  parameters,
		Fn:              fn,
	}
}

// ParseToolInput is a helper to parse JSON input into a typed struct.
// This uses generics for type-safe parsing of tool arguments.
//
// Example:
//
//	type SearchParams struct {
//	    Query string `json:"query"`
//	    Limit int    `json:"limit,omitempty"`
//	}
//
//	params, err := ParseToolInput[SearchParams](input)
//	if err != nil {
//	    return "", fmt.Errorf("invalid input: %w", err)
//	}
func ParseToolInput[T any](input string) (T, error) {
	var result T
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return result, err
	}
	return result, nil
}

// FormatToolOutput is a helper to format output as JSON.
// This serializes any value to a JSON string for returning to the LLM.
//
// Example:
//
//	result := map[string]any{
//	    "status": "success",
//	    "count":  42,
//	}
//	output, err := FormatToolOutput(result)
func FormatToolOutput(output any) (string, error) {
	data, err := json.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
