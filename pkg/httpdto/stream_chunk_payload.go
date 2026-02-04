package httpdto

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
)

type StreamChunkPayload struct {
	Type      string               `json:"type"`
	Content   string               `json:"content,omitempty"`
	Citation  *domain.Citation     `json:"citation,omitempty"`
	Usage     *services.TokenUsage `json:"usage,omitempty"`
	Error     string               `json:"error,omitempty"`
	Timestamp int64                `json:"timestamp,omitempty"`
}
