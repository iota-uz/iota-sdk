package rpc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTurns_UserAssistantSetsAssistantRole(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	base := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	msgs := []types.Message{
		types.UserMessage("question",
			types.WithSessionID(sessionID),
			types.WithCreatedAt(base),
		),
		types.AssistantMessage("answer",
			types.WithSessionID(sessionID),
			types.WithCreatedAt(base.Add(time.Second)),
		),
	}

	turns := buildTurns(msgs)
	require.Len(t, turns, 1)
	require.NotNil(t, turns[0].AssistantTurn)
	assert.Equal(t, "assistant", turns[0].AssistantTurn.Role)
	assert.Equal(t, "question", turns[0].UserTurn.Content)
	assert.Equal(t, "answer", turns[0].AssistantTurn.Content)
}

func TestBuildTurns_SystemMessageMappedInline(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	msg := types.SystemMessage("compacted summary",
		types.WithSessionID(sessionID),
		types.WithCreatedAt(time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)),
	)

	turns := buildTurns([]types.Message{msg})
	require.Len(t, turns, 1)
	require.NotNil(t, turns[0].AssistantTurn)
	assert.Empty(t, turns[0].UserTurn.Content)
	assert.Equal(t, "system", turns[0].AssistantTurn.Role)
	assert.Equal(t, "compacted summary", turns[0].AssistantTurn.Content)
}

func TestBuildTurns_MixedHistoryKeepsSystemTurn(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	base := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	msgs := []types.Message{
		types.UserMessage("u1", types.WithSessionID(sessionID), types.WithCreatedAt(base)),
		types.AssistantMessage("a1", types.WithSessionID(sessionID), types.WithCreatedAt(base.Add(time.Second))),
		types.SystemMessage("summary", types.WithSessionID(sessionID), types.WithCreatedAt(base.Add(2*time.Second))),
		types.UserMessage("u2", types.WithSessionID(sessionID), types.WithCreatedAt(base.Add(3*time.Second))),
		types.AssistantMessage("a2", types.WithSessionID(sessionID), types.WithCreatedAt(base.Add(4*time.Second))),
	}

	turns := buildTurns(msgs)
	require.Len(t, turns, 3)
	require.NotNil(t, turns[0].AssistantTurn)
	require.NotNil(t, turns[1].AssistantTurn)
	require.NotNil(t, turns[2].AssistantTurn)
	assert.Equal(t, "u1", turns[0].UserTurn.Content)
	assert.Equal(t, "assistant", turns[0].AssistantTurn.Role)
	assert.Equal(t, "summary", turns[1].AssistantTurn.Content)
	assert.Equal(t, "system", turns[1].AssistantTurn.Role)
	assert.Empty(t, turns[1].UserTurn.Content)
	assert.Equal(t, "u2", turns[2].UserTurn.Content)
	assert.Equal(t, "assistant", turns[2].AssistantTurn.Role)
}

func TestBuildTurns_AssistantDebugTraceMapped(t *testing.T) {
	t.Parallel()

	sessionID := uuid.New()
	base := time.Date(2026, 2, 6, 10, 0, 0, 0, time.UTC)
	msgs := []types.Message{
		types.UserMessage("question", types.WithSessionID(sessionID), types.WithCreatedAt(base)),
		types.AssistantMessage(
			"answer",
			types.WithSessionID(sessionID),
			types.WithCreatedAt(base.Add(time.Second)),
			types.WithDebugTrace(&types.DebugTrace{
				Usage: &types.DebugUsage{
					PromptTokens:     100,
					CompletionTokens: 35,
					TotalTokens:      135,
					CachedTokens:     20,
				},
				GenerationMs: 1200,
				Tools: []types.DebugToolCall{
					{
						CallID:     "call_1",
						Name:       "sql_execute",
						Arguments:  `{"query":"select 1"}`,
						Result:     `{"rows":[{"value":1}]}`,
						DurationMs: 80,
					},
				},
			}),
		),
	}

	turns := buildTurns(msgs)
	require.Len(t, turns, 1)
	require.NotNil(t, turns[0].AssistantTurn)
	require.NotNil(t, turns[0].AssistantTurn.Debug)
	require.NotNil(t, turns[0].AssistantTurn.Debug.Usage)
	assert.Equal(t, 100, turns[0].AssistantTurn.Debug.Usage.PromptTokens)
	assert.Equal(t, 35, turns[0].AssistantTurn.Debug.Usage.CompletionTokens)
	assert.Equal(t, int64(1200), turns[0].AssistantTurn.Debug.GenerationMs)
	require.Len(t, turns[0].AssistantTurn.Debug.Tools, 1)
	assert.Equal(t, "call_1", turns[0].AssistantTurn.Debug.Tools[0].CallID)
}
