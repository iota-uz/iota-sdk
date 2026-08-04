package domain

import (
	"testing"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

func TestNewAssistantMessage_NormalizesInvalidUTF8AcrossPersistedPayload(t *testing.T) {
	t.Parallel()

	invalid := "Ответ " + string([]byte{0xd0}) + "."
	trace := &types.DebugTrace{
		TraceID:  invalid,
		Thinking: invalid,
		Tools: []types.DebugToolCall{{
			CallID: invalid,
			Result: invalid,
		}},
		Attempts: []types.DebugGeneration{{
			ID:       invalid,
			Output:   invalid,
			Thinking: invalid,
			ToolCalls: []types.DebugToolCall{{
				Error: invalid,
			}},
		}},
		Spans: []types.DebugSpan{{
			ID:     invalid,
			Output: invalid,
		}},
		Events: []types.DebugEvent{{
			ID:      invalid,
			Message: invalid,
		}},
	}

	msg, err := NewAssistantMessage(AssistantMessageSpec{
		SessionID: uuid.New(),
		Content:   invalid,
		ToolCalls: []types.ToolCall{{
			ID:     invalid,
			Result: invalid,
		}},
		DebugTrace: trace,
	})
	require.NoError(t, err)
	require.True(t, utf8.ValidString(msg.Content()))
	require.Contains(t, msg.Content(), invalidUTF8Replacement)
	require.True(t, utf8.ValidString(msg.ToolCalls()[0].ID))
	require.True(t, utf8.ValidString(msg.ToolCalls()[0].Result))

	normalizedTrace := msg.DebugTrace()
	require.NotNil(t, normalizedTrace)
	require.True(t, utf8.ValidString(normalizedTrace.TraceID))
	require.True(t, utf8.ValidString(normalizedTrace.Thinking))
	require.True(t, utf8.ValidString(normalizedTrace.Tools[0].CallID))
	require.True(t, utf8.ValidString(normalizedTrace.Tools[0].Result))
	require.True(t, utf8.ValidString(normalizedTrace.Attempts[0].ID))
	require.True(t, utf8.ValidString(normalizedTrace.Attempts[0].Output))
	require.True(t, utf8.ValidString(normalizedTrace.Attempts[0].Thinking))
	require.True(t, utf8.ValidString(normalizedTrace.Attempts[0].ToolCalls[0].Error))
	require.True(t, utf8.ValidString(normalizedTrace.Spans[0].ID))
	require.True(t, utf8.ValidString(normalizedTrace.Spans[0].Output))
	require.True(t, utf8.ValidString(normalizedTrace.Events[0].ID))
	require.True(t, utf8.ValidString(normalizedTrace.Events[0].Message))

	require.Equal(t, invalid, trace.TraceID, "normalization must not mutate the caller's trace")
}
