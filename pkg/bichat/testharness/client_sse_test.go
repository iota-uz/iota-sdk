package testharness

import (
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/httpdto"
	"github.com/stretchr/testify/require"
)

func TestDecodeSSE_EventLinesOptional(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"event: content",
		"data: {\"type\":\"content\",\"content\":\"Hi\"}",
		"",
		"event: done",
		"data: {\"type\":\"done\"}",
		"",
	}, "\n")

	var got []httpdto.StreamChunkPayload
	err := decodeSSE(strings.NewReader(input), func(p httpdto.StreamChunkPayload) error {
		got = append(got, p)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "content", got[0].Type)
	require.Equal(t, "Hi", got[0].Content)
	require.Equal(t, "done", got[1].Type)
}

func TestDecodeSSE_DataOnly(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"data:{\"type\":\"content\",\"content\":\"A\"}",
		"",
		"data: {\"type\":\"content\",\"content\":\"B\"}",
		"",
		"data: {\"type\":\"done\"}",
		"",
	}, "\n")

	var b strings.Builder
	err := decodeSSE(strings.NewReader(input), func(p httpdto.StreamChunkPayload) error {
		if p.Type == "content" {
			b.WriteString(p.Content)
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, "AB", b.String())
}

func TestMapSSEInterrupt(t *testing.T) {
	t.Parallel()

	interrupt := mapSSEInterrupt(&httpdto.InterruptEventPayload{
		CheckpointID:       "cp-1",
		AgentName:          "analytics-agent",
		ProviderResponseID: "resp-1",
		Questions: []httpdto.InterruptQuestionPayload{
			{
				ID:   "metric",
				Text: "Which metric?",
				Type: "single_choice",
				Options: []httpdto.InterruptQuestionOptionPayload{
					{ID: "revenue", Label: "Revenue"},
					{ID: "profit", Label: "Profit"},
				},
			},
		},
	})

	require.NotNil(t, interrupt)
	require.Equal(t, "cp-1", interrupt.CheckpointID)
	require.Equal(t, "analytics-agent", interrupt.AgentName)
	require.Equal(t, "resp-1", interrupt.ProviderResponseID)
	require.Len(t, interrupt.Questions, 1)
	require.Equal(t, "metric", interrupt.Questions[0].ID)
	require.Len(t, interrupt.Questions[0].Options, 2)
	require.Equal(t, "revenue", interrupt.Questions[0].Options[0].ID)
}
