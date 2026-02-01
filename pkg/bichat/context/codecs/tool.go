package codecs

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/bichat/context"
)

// ToolSchemaPayload represents a tool schema block.
type ToolSchemaPayload struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolSchemaCodec handles tool schema blocks.
type ToolSchemaCodec struct {
	*context.BaseCodec
}

// NewToolSchemaCodec creates a new tool schema codec.
func NewToolSchemaCodec() *ToolSchemaCodec {
	return &ToolSchemaCodec{
		BaseCodec: context.NewBaseCodec("tool-schema", "1.0.0"),
	}
}

// Validate validates the tool schema payload.
func (c *ToolSchemaCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case ToolSchemaPayload:
		if v.Name == "" {
			return fmt.Errorf("tool schema name cannot be empty")
		}
		if v.Description == "" {
			return fmt.Errorf("tool schema description cannot be empty")
		}
		return nil
	case map[string]any:
		if name, ok := v["name"].(string); !ok || name == "" {
			return fmt.Errorf("tool schema name cannot be empty")
		}
		if desc, ok := v["description"].(string); !ok || desc == "" {
			return fmt.Errorf("tool schema description cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid tool schema payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *ToolSchemaCodec) Canonicalize(payload any) ([]byte, error) {
	var schema ToolSchemaPayload

	switch v := payload.(type) {
	case ToolSchemaPayload:
		schema = v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			schema.Name = name
		}
		if desc, ok := v["description"].(string); ok {
			schema.Description = normalizeWhitespace(desc)
		}
		if params, ok := v["parameters"].(map[string]any); ok {
			schema.Parameters = params
		}
	default:
		return nil, fmt.Errorf("invalid tool schema payload type: %T", payload)
	}

	return context.SortedJSONBytes(schema)
}

// ToolOutputPayload represents a tool execution result block.
type ToolOutputPayload struct {
	ToolName string `json:"tool_name"`
	Input    string `json:"input"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
}

// ToolOutputCodec handles tool output blocks.
type ToolOutputCodec struct {
	*context.BaseCodec
}

// NewToolOutputCodec creates a new tool output codec.
func NewToolOutputCodec() *ToolOutputCodec {
	return &ToolOutputCodec{
		BaseCodec: context.NewBaseCodec("tool-output", "1.0.0"),
	}
}

// Validate validates the tool output payload.
func (c *ToolOutputCodec) Validate(payload any) error {
	switch v := payload.(type) {
	case ToolOutputPayload:
		if v.ToolName == "" {
			return fmt.Errorf("tool output tool_name cannot be empty")
		}
		return nil
	case map[string]any:
		if name, ok := v["tool_name"].(string); !ok || name == "" {
			return fmt.Errorf("tool output tool_name cannot be empty")
		}
		return nil
	default:
		return fmt.Errorf("invalid tool output payload type: %T", payload)
	}
}

// Canonicalize converts the payload to canonical form.
func (c *ToolOutputCodec) Canonicalize(payload any) ([]byte, error) {
	var output ToolOutputPayload

	switch v := payload.(type) {
	case ToolOutputPayload:
		output = v
	case map[string]any:
		if name, ok := v["tool_name"].(string); ok {
			output.ToolName = name
		}
		if input, ok := v["input"].(string); ok {
			output.Input = input
		}
		if result, ok := v["output"].(string); ok {
			output.Output = result
		}
		if err, ok := v["error"].(string); ok {
			output.Error = err
		}
	default:
		return nil, fmt.Errorf("invalid tool output payload type: %T", payload)
	}

	return context.SortedJSONBytes(output)
}
