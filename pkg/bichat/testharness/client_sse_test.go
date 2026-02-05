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
