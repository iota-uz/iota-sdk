package httpdto

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

type ToolEventPayload struct {
	CallID     string `json:"callId,omitempty"`
	Name       string `json:"name,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type StreamChunkPayload struct {
	Type         string            `json:"type"`
	Content      string            `json:"content,omitempty"`
	Citation     *domain.Citation  `json:"citation,omitempty"`
	Usage        *types.DebugUsage `json:"usage,omitempty"`
	Tool         *ToolEventPayload `json:"tool,omitempty"`
	GenerationMs int64             `json:"generationMs,omitempty"`
	Error        string            `json:"error,omitempty"`
	Timestamp    int64             `json:"timestamp,omitempty"`
}
