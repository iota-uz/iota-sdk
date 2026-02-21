package tools

import (
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/context/formatters"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// FormatStructuredResult formats a ToolResult from CallStructured into a string.
// This is the canonical Call() implementation for StructuredTool wrappers.
//
// It handles:
//   - Formatting the result payload via the default formatter registry
//   - ErrStructuredToolOutput sentinel (returns formatted string with nil error)
//   - Non-nil errors with a result (returns formatted string with original error)
//   - Fallback to JSON serialization when no formatter is registered
func FormatStructuredResult(result *types.ToolResult, err error) (string, error) {
	if err != nil {
		if result != nil {
			registry := formatters.DefaultFormatterRegistry()
			if f := registry.Get(result.CodecID); f != nil {
				formatted, fmtErr := f.Format(result.Payload, types.DefaultFormatOptions())
				if fmtErr == nil {
					if errors.Is(err, agents.ErrStructuredToolOutput) {
						return formatted, nil
					}
					return formatted, err
				}
			}
		}
		return "", err
	}
	if result == nil {
		return "", nil
	}
	registry := formatters.DefaultFormatterRegistry()
	f := registry.Get(result.CodecID)
	if f == nil {
		return agents.FormatToolOutput(result.Payload)
	}
	return f.Format(result.Payload, types.DefaultFormatOptions())
}

