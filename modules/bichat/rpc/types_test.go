package rpc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPendingQuestionFromMessages_SelectsLatestPending(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	baseTime := time.Now().UTC()

	oldPending := &types.QuestionData{
		CheckpointID: "checkpoint-old",
		Status:       types.QuestionStatusPending,
		AgentName:    "planner",
		Questions: []types.QuestionDataItem{
			{
				ID:   "q-old",
				Text: "Old question",
				Type: "SINGLE_CHOICE",
				Options: []types.QuestionDataOption{
					{ID: "opt-1", Label: "One"},
					{ID: "opt-2", Label: "Two"},
				},
			},
		},
	}

	newPending := &types.QuestionData{
		CheckpointID: "checkpoint-new",
		Status:       types.QuestionStatusPending,
		AgentName:    "planner",
		Questions: []types.QuestionDataItem{
			{
				ID:   "q-new",
				Text: "New question",
				Type: "MULTIPLE_CHOICE",
				Options: []types.QuestionDataOption{
					{ID: "opt-a", Label: "A"},
					{ID: "opt-b", Label: "B"},
				},
			},
		},
	}

	userTurnBeforeLatest := types.UserMessage(
		"latest user turn",
		types.WithSessionID(sessionID),
		types.WithCreatedAt(baseTime.Add(2*time.Second)),
	)

	messages := []types.Message{
		types.UserMessage(
			"first user turn",
			types.WithSessionID(sessionID),
			types.WithCreatedAt(baseTime),
		),
		types.AssistantMessage(
			"old pending",
			types.WithSessionID(sessionID),
			types.WithQuestionData(oldPending),
			types.WithCreatedAt(baseTime.Add(1*time.Second)),
		),
		userTurnBeforeLatest,
		types.AssistantMessage(
			"latest pending",
			types.WithSessionID(sessionID),
			types.WithQuestionData(newPending),
			types.WithCreatedAt(baseTime.Add(3*time.Second)),
		),
	}

	pending := pendingQuestionFromMessages(messages)
	require.NotNil(t, pending)
	assert.Equal(t, "checkpoint-new", pending.CheckpointID)
	assert.Equal(t, userTurnBeforeLatest.ID().String(), pending.TurnID)
	require.Len(t, pending.Questions, 1)
	assert.Equal(t, "q-new", pending.Questions[0].ID)
	assert.Equal(t, "MULTIPLE_CHOICE", pending.Questions[0].Type)
}

func TestPendingQuestionFromMessages_ReturnsNilWhenNoPending(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	baseTime := time.Now().UTC()

	answered := &types.QuestionData{
		CheckpointID: "checkpoint-answered",
		Status:       types.QuestionStatusAnswered,
		Questions: []types.QuestionDataItem{
			{
				ID:   "q1",
				Text: "Answered question",
				Type: "SINGLE_CHOICE",
				Options: []types.QuestionDataOption{
					{ID: "opt-1", Label: "One"},
					{ID: "opt-2", Label: "Two"},
				},
			},
		},
	}

	messages := []types.Message{
		types.UserMessage("hello", types.WithSessionID(sessionID), types.WithCreatedAt(baseTime)),
		types.AssistantMessage(
			"answered question",
			types.WithSessionID(sessionID),
			types.WithQuestionData(answered),
			types.WithCreatedAt(baseTime.Add(1*time.Second)),
		),
	}

	pending := pendingQuestionFromMessages(messages)
	assert.Nil(t, pending)
}
