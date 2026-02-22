package httpdto

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

type ToolEventPayload struct {
	CallID     string `json:"callId,omitempty"`
	Name       string `json:"name,omitempty"`
	AgentName  string `json:"agentName,omitempty"`
	Arguments  string `json:"arguments,omitempty"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

type InterruptQuestionOptionPayload struct {
	ID          string `json:"id,omitempty"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}

type InterruptQuestionPayload struct {
	ID      string                           `json:"id,omitempty"`
	Text    string                           `json:"text,omitempty"`
	Type    string                           `json:"type,omitempty"`
	Options []InterruptQuestionOptionPayload `json:"options,omitempty"`
}

type InterruptEventPayload struct {
	CheckpointID       string                     `json:"checkpointId,omitempty"`
	AgentName          string                     `json:"agentName,omitempty"`
	ProviderResponseID string                     `json:"providerResponseId,omitempty"`
	Questions          []InterruptQuestionPayload `json:"questions,omitempty"`
}

// StreamSnapshotPayload is the partial state sent when resuming a stream.
type StreamSnapshotPayload struct {
	PartialContent  string         `json:"partialContent,omitempty"`
	PartialMetadata map[string]any `json:"partialMetadata,omitempty"`
}

type StreamChunkPayload struct {
	Type         string                 `json:"type"`
	Content      string                 `json:"content,omitempty"`
	Citation     *types.Citation        `json:"citation,omitempty"`
	Usage        *types.DebugUsage      `json:"usage,omitempty"`
	Tool         *ToolEventPayload      `json:"tool,omitempty"`
	Interrupt    *InterruptEventPayload `json:"interrupt,omitempty"`
	GenerationMs int64                  `json:"generationMs,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Timestamp    int64                  `json:"timestamp,omitempty"`
	Snapshot     *StreamSnapshotPayload `json:"snapshot,omitempty"`
	RunID        string                 `json:"runId,omitempty"`
}
