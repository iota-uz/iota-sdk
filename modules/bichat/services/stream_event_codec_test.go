package services

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/httpdto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeRunEventFromChunk_Content(t *testing.T) {
	t.Parallel()

	chunk := bichatservices.StreamChunk{
		Type:      bichatservices.ChunkTypeContent,
		Content:   "hello",
		Timestamp: time.UnixMilli(1_700_000_000_000),
	}
	eventType, body, err := encodeRunEventFromChunk(chunk)
	require.NoError(t, err)
	assert.Equal(t, "content", eventType)

	var decoded httpdto.StreamChunkPayload
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "content", decoded.Type)
	assert.Equal(t, "hello", decoded.Content)
	assert.Equal(t, int64(1_700_000_000_000), decoded.Timestamp)
}

func TestEncodeRunEventFromChunk_TextBlockEndCarriesSeq(t *testing.T) {
	t.Parallel()

	chunk := bichatservices.StreamChunk{
		Type:         bichatservices.ChunkTypeTextBlockEnd,
		TextBlockSeq: 2,
		Timestamp:    time.Now(),
	}
	eventType, body, err := encodeRunEventFromChunk(chunk)
	require.NoError(t, err)
	assert.Equal(t, "text_block_end", eventType)

	var decoded httpdto.StreamChunkPayload
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.NotNil(t, decoded.TextBlockSeq, "text_block_end must surface the seq number")
	assert.Equal(t, 2, *decoded.TextBlockSeq)
}

func TestEncodeRunEventFromChunk_ToolAndError(t *testing.T) {
	t.Parallel()

	chunk := bichatservices.StreamChunk{
		Type: bichatservices.ChunkTypeToolEnd,
		Tool: &bichatservices.ToolEvent{
			CallID:     "c1",
			Name:       "sql_execute",
			Arguments:  `{"query":"SELECT 1"}`,
			Result:     "[[1]]",
			Error:      errors.New("timeout"),
			DurationMs: 42,
		},
		Timestamp: time.Now(),
	}
	_, body, err := encodeRunEventFromChunk(chunk)
	require.NoError(t, err)

	var decoded httpdto.StreamChunkPayload
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.NotNil(t, decoded.Tool)
	assert.Equal(t, "c1", decoded.Tool.CallID)
	assert.Equal(t, "sql_execute", decoded.Tool.Name)
	assert.Equal(t, "timeout", decoded.Tool.Error)
	assert.EqualValues(t, 42, decoded.Tool.DurationMs)
}

func TestEncodeRunEventFromChunk_SnapshotRoundTrip(t *testing.T) {
	t.Parallel()

	chunk := bichatservices.StreamChunk{
		Type: bichatservices.ChunkTypeSnapshot,
		Snapshot: &bichatservices.StreamSnapshot{
			PartialContent:  "hi",
			PartialMetadata: map[string]any{"tool_calls": []types.ToolCall{}},
		},
	}
	_, body, err := encodeRunEventFromChunk(chunk)
	require.NoError(t, err)

	var decoded httpdto.StreamChunkPayload
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.NotNil(t, decoded.Snapshot)
	assert.Equal(t, "hi", decoded.Snapshot.PartialContent)
}

func TestEncodeRunEventFromChunk_EmptyTypeFallsBackToChunk(t *testing.T) {
	t.Parallel()

	eventType, _, err := encodeRunEventFromChunk(bichatservices.StreamChunk{})
	require.NoError(t, err)
	assert.Equal(t, "chunk", eventType, "empty chunk type must fall back to the controller default")
}
