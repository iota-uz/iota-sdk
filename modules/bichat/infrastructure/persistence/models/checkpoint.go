package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// CheckpointModel is the database model for bichat.checkpoints.
type CheckpointModel struct {
	ID                 string
	TenantID           uuid.UUID
	UserID             uint
	SessionID          uuid.UUID
	ThreadID           string
	AgentName          string
	Messages           []byte
	PendingTools       []byte
	InterruptType      string
	InterruptData      []byte
	PreviousResponseID *string
	CreatedAt          time.Time
}

// checkpointJSON is an intermediary for assembling a JSON blob that
// Checkpoint.UnmarshalJSON knows how to decode. This reuses the domain's
// custom unmarshaler which handles the Message interface via messageDTO.
type checkpointJSON struct {
	ID                 string          `json:"id"`
	TenantID           uuid.UUID       `json:"tenant_id"`
	SessionID          uuid.UUID       `json:"session_id"`
	ThreadID           string          `json:"thread_id"`
	AgentName          string          `json:"agent_name"`
	Messages           json.RawMessage `json:"messages"`
	PendingTools       json.RawMessage `json:"pending_tools"`
	InterruptType      string          `json:"interrupt_type"`
	InterruptData      json.RawMessage `json:"interrupt_data,omitempty"`
	PreviousResponseID *string         `json:"previous_response_id,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
}

// ToDomain converts the database model to a domain Checkpoint.
// It delegates to Checkpoint.UnmarshalJSON which handles the Message
// interface deserialization via its internal messageDTO pattern.
func (m *CheckpointModel) ToDomain() (*agents.Checkpoint, error) {
	blob, err := json.Marshal(checkpointJSON{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		SessionID:          m.SessionID,
		ThreadID:           m.ThreadID,
		AgentName:          m.AgentName,
		Messages:           m.Messages,
		PendingTools:       m.PendingTools,
		InterruptType:      m.InterruptType,
		InterruptData:      m.InterruptData,
		PreviousResponseID: m.PreviousResponseID,
		CreatedAt:          m.CreatedAt,
	})
	if err != nil {
		return nil, err
	}
	var cp agents.Checkpoint
	if err := json.Unmarshal(blob, &cp); err != nil {
		return nil, err
	}
	return &cp, nil
}

// CheckpointModelFromDomain converts a domain Checkpoint to the database model.
// It delegates to Checkpoint.MarshalJSON which serializes Message interfaces
// through its internal messageDTO pattern (unexported fields require this).
func CheckpointModelFromDomain(cp *agents.Checkpoint, userID uint) (*CheckpointModel, error) {
	blob, err := json.Marshal(cp)
	if err != nil {
		return nil, err
	}
	var dto checkpointJSON
	if err := json.Unmarshal(blob, &dto); err != nil {
		return nil, err
	}

	return &CheckpointModel{
		ID:                 dto.ID,
		TenantID:           dto.TenantID,
		UserID:             userID,
		SessionID:          dto.SessionID,
		ThreadID:           dto.ThreadID,
		AgentName:          dto.AgentName,
		Messages:           dto.Messages,
		PendingTools:       dto.PendingTools,
		InterruptType:      dto.InterruptType,
		InterruptData:      dto.InterruptData,
		PreviousResponseID: dto.PreviousResponseID,
		CreatedAt:          dto.CreatedAt,
	}, nil
}
