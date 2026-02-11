package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckpointModel_ToDomain_MessageInterface is a regression test for the
// "json: cannot unmarshal object into Go value of type types.Message" bug.
//
// types.Message is an interface. A plain json.Unmarshal into []types.Message
// fails because the JSON decoder doesn't know which concrete type to use.
// ToDomain must delegate to Checkpoint.UnmarshalJSON which uses messageDTO.
func TestCheckpointModel_ToDomain_MessageInterface(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	msgID := uuid.New()

	// Build messages JSON the same way the DB stores it: via CheckpointModelFromDomain
	// which delegates to Checkpoint.MarshalJSON (serializes Message interfaces through messageDTO).
	msg := types.NewMessage(
		types.WithMessageID(msgID),
		types.WithSessionID(sessionID),
		types.WithRole(types.RoleUser),
		types.WithContent("What are total sales?"),
	)
	cp := agents.NewCheckpoint("thread-1", "bi-agent", []types.Message{msg},
		agents.WithSessionID(sessionID),
		agents.WithTenantID(uuid.New()),
		agents.WithInterruptType("ask_user_question"),
		agents.WithInterruptData(json.RawMessage(`{"questions":[{"id":"q1","text":"Which year?"}]}`)),
	)

	m, err := CheckpointModelFromDomain(cp, 1)
	require.NoError(t, err)

	got, err := m.ToDomain()
	require.NoError(t, err, "ToDomain must handle Message interface deserialization")

	assert.Equal(t, cp.ID, got.ID)
	assert.Equal(t, cp.ThreadID, got.ThreadID)
	assert.Equal(t, cp.AgentName, got.AgentName)
	assert.Equal(t, cp.SessionID, got.SessionID)
	assert.Equal(t, cp.InterruptType, got.InterruptType)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, msgID, got.Messages[0].ID())
	assert.Equal(t, types.RoleUser, got.Messages[0].Role())
	assert.Equal(t, "What are total sales?", got.Messages[0].Content())
}

// TestCheckpointModel_ToDomain_NullableInterruptData is a regression test for the
// "can't scan into dest[8]: json: cannot unmarshal object into Go value of type []uint8" bug.
//
// The interrupt_data column is nullable JSONB. When it's NULL, the model's
// InterruptData field is nil ([]byte). When it's a JSON object, it must round-trip
// correctly through ToDomain without scan errors (the original bug used *[]byte
// which caused pgx to attempt JSON unmarshaling instead of returning raw bytes).
func TestCheckpointModel_ToDomain_NullableInterruptData(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	msg := types.NewMessage(
		types.WithRole(types.RoleUser),
		types.WithContent("hello"),
		types.WithSessionID(sessionID),
	)

	t.Run("NilInterruptData", func(t *testing.T) {
		t.Parallel()
		cp := agents.NewCheckpoint("thread-1", "agent", []types.Message{msg},
			agents.WithSessionID(sessionID),
			agents.WithTenantID(uuid.New()),
		)
		m, err := CheckpointModelFromDomain(cp, 1)
		require.NoError(t, err)
		// Simulate NULL from DB by clearing the field.
		m.InterruptData = nil

		got, err := m.ToDomain()
		require.NoError(t, err)
		assert.Nil(t, got.InterruptData)
	})

	t.Run("ObjectInterruptData", func(t *testing.T) {
		t.Parallel()
		interruptJSON := json.RawMessage(`{"type":"ask_user_question","questions":[{"id":"q1","text":"Which region?"}]}`)

		cp := agents.NewCheckpoint("thread-2", "agent", []types.Message{msg},
			agents.WithSessionID(sessionID),
			agents.WithTenantID(uuid.New()),
			agents.WithInterruptType("ask_user_question"),
			agents.WithInterruptData(interruptJSON),
		)
		m, err := CheckpointModelFromDomain(cp, 1)
		require.NoError(t, err)

		got, err := m.ToDomain()
		require.NoError(t, err)
		assert.JSONEq(t, string(interruptJSON), string(got.InterruptData))
	})
}

// TestCheckpointModel_RoundTrip verifies that FromDomain -> ToDomain preserves all fields.
func TestCheckpointModel_RoundTrip(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	tenantID := uuid.New()
	prevResp := "resp-abc"

	msgs := []types.Message{
		types.NewMessage(
			types.WithRole(types.RoleUser),
			types.WithContent("query 1"),
			types.WithSessionID(sessionID),
		),
		types.NewMessage(
			types.WithRole(types.RoleAssistant),
			types.WithContent("result 1"),
			types.WithSessionID(sessionID),
		),
	}

	original := agents.NewCheckpoint("thread-rt", "bi-agent", msgs,
		agents.WithSessionID(sessionID),
		agents.WithTenantID(tenantID),
		agents.WithCheckpointPreviousResponseID(&prevResp),
		agents.WithPendingTools([]types.ToolCall{
			{ID: "tc-1", Name: "sql_execute", Arguments: `{"query":"SELECT 1"}`},
		}),
		agents.WithInterruptType("ask_user_question"),
		agents.WithInterruptData(json.RawMessage(`{"questions":[]}`)),
	)

	m, err := CheckpointModelFromDomain(original, 42)
	require.NoError(t, err)
	assert.Equal(t, uint(42), m.UserID)

	restored, err := m.ToDomain()
	require.NoError(t, err)

	assert.Equal(t, original.ID, restored.ID)
	assert.Equal(t, original.ThreadID, restored.ThreadID)
	assert.Equal(t, original.AgentName, restored.AgentName)
	assert.Equal(t, original.SessionID, restored.SessionID)
	assert.Equal(t, original.TenantID, restored.TenantID)
	assert.Equal(t, original.InterruptType, restored.InterruptType)
	assert.JSONEq(t, string(original.InterruptData), string(restored.InterruptData))
	require.NotNil(t, restored.PreviousResponseID)
	assert.Equal(t, prevResp, *restored.PreviousResponseID)

	require.Len(t, restored.Messages, 2)
	assert.Equal(t, "query 1", restored.Messages[0].Content())
	assert.Equal(t, types.RoleUser, restored.Messages[0].Role())
	assert.Equal(t, "result 1", restored.Messages[1].Content())
	assert.Equal(t, types.RoleAssistant, restored.Messages[1].Role())

	require.Len(t, restored.PendingTools, 1)
	assert.Equal(t, "tc-1", restored.PendingTools[0].ID)
	assert.Equal(t, "sql_execute", restored.PendingTools[0].Name)

	assert.WithinDuration(t, original.CreatedAt, restored.CreatedAt, time.Second, "CreatedAt should round-trip")
}
