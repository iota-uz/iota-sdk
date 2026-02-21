package llmproviders

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/openai/openai-go/v3/responses"
)

// toolCallAccumEntry accumulates streaming function call data.
type toolCallAccumEntry struct {
	id     string // item ID (used as map key)
	callID string // function call ID (used in API)
	name   string
	args   string
}

// buildToolCallsFromAccum converts accumulated tool call data to types.ToolCall slice.
func (m *OpenAIModel) buildToolCallsFromAccum(accum map[string]*toolCallAccumEntry, order []string) []types.ToolCall {
	if len(accum) == 0 {
		return nil
	}
	merged := make(map[string]types.ToolCall, len(accum))
	callOrder := make([]string, 0, len(accum))
	for _, key := range order {
		if a, ok := accum[key]; ok {
			id := strings.TrimSpace(a.callID)
			if id == "" {
				id = strings.TrimSpace(a.id)
			}
			name := strings.TrimSpace(a.name)
			if id == "" || name == "" {
				continue
			}

			if _, exists := merged[id]; !exists {
				callOrder = append(callOrder, id)
			}

			merged[id] = types.ToolCall{
				ID:        id,
				Name:      name,
				Arguments: a.args,
			}
		}
	}

	calls := make([]types.ToolCall, 0, len(callOrder))
	for _, callID := range callOrder {
		calls = append(calls, merged[callID])
	}

	return calls
}

// buildReadyToolCallsFromAccum returns tool calls that are ready to execute during streaming.
// A tool call is considered ready once we have a stable CallID and Name.
func (m *OpenAIModel) buildReadyToolCallsFromAccum(accum map[string]*toolCallAccumEntry, order []string) []types.ToolCall {
	if len(accum) == 0 {
		return nil
	}
	calls := make([]types.ToolCall, 0, len(accum))
	seen := make(map[string]struct{}, len(accum))
	for _, key := range order {
		a, ok := accum[key]
		if !ok {
			continue
		}
		callID := strings.TrimSpace(a.callID)
		name := strings.TrimSpace(a.name)
		if callID == "" || name == "" {
			continue
		}
		if _, exists := seen[callID]; exists {
			continue
		}
		seen[callID] = struct{}{}
		calls = append(calls, types.ToolCall{
			ID:        callID,
			Name:      name,
			Arguments: a.args,
		})
	}
	return calls
}

func functionCallItemKey(item responses.ResponseOutputItemUnion, fallback string) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return id
	}
	if id := strings.TrimSpace(fallback); id != "" {
		return id
	}
	if callID := strings.TrimSpace(item.CallID); callID != "" {
		return callID
	}
	return ""
}
