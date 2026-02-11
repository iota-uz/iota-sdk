package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
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

// ToDomain converts the database model to a domain Checkpoint.
func (m *CheckpointModel) ToDomain() (*agents.Checkpoint, error) {
	var msgs []types.Message
	if err := json.Unmarshal(m.Messages, &msgs); err != nil {
		return nil, err
	}

	var tools []types.ToolCall
	if err := json.Unmarshal(m.PendingTools, &tools); err != nil {
		return nil, err
	}

	cp := &agents.Checkpoint{
		ID:                 m.ID,
		TenantID:           m.TenantID,
		SessionID:          m.SessionID,
		ThreadID:           m.ThreadID,
		AgentName:          m.AgentName,
		Messages:           msgs,
		PendingTools:       tools,
		InterruptType:      m.InterruptType,
		InterruptData:      m.InterruptData,
		PreviousResponseID: m.PreviousResponseID,
		CreatedAt:          m.CreatedAt,
	}
	return cp, nil
}

// CheckpointModelFromDomain converts a domain Checkpoint to the database model.
func CheckpointModelFromDomain(cp *agents.Checkpoint, userID uint) (*CheckpointModel, error) {
	messagesJSON, err := json.Marshal(cp.Messages)
	if err != nil {
		return nil, err
	}

	pendingToolsJSON, err := json.Marshal(cp.PendingTools)
	if err != nil {
		return nil, err
	}

	return &CheckpointModel{
		ID:                 cp.ID,
		TenantID:           cp.TenantID,
		UserID:             userID,
		SessionID:          cp.SessionID,
		ThreadID:           cp.ThreadID,
		AgentName:          cp.AgentName,
		Messages:           messagesJSON,
		PendingTools:       pendingToolsJSON,
		InterruptType:      cp.InterruptType,
		InterruptData:      cp.InterruptData,
		PreviousResponseID: cp.PreviousResponseID,
		CreatedAt:          cp.CreatedAt,
	}, nil
}
