package testharness

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
	"github.com/stretchr/testify/require"
)

func TestExtractChatMessageContent_FromStringAndArray(t *testing.T) {
	t.Parallel()

	msg := openai.ChatCompletionMessage{Content: "plain text"}
	require.Equal(t, "plain text", extractChatMessageContent(msg))

	var arrayMsg openai.ChatCompletionMessage
	err := json.Unmarshal([]byte(`{
		"role": "assistant",
		"content": [
			{"type":"output_text","text":"line 1"},
			{"type":"output_text","text":"line 2"}
		],
		"refusal": ""
	}`), &arrayMsg)
	require.NoError(t, err)
	require.Equal(t, "line 1\nline 2", extractChatMessageContent(arrayMsg))
}

func TestExtractChatMessageRefusal_FromArray(t *testing.T) {
	t.Parallel()

	var msg openai.ChatCompletionMessage
	err := json.Unmarshal([]byte(`{
		"role": "assistant",
		"content": "",
		"refusal": [{"type":"refusal","text":"policy blocked"}]
	}`), &msg)
	require.NoError(t, err)
	require.Equal(t, "policy blocked", extractChatMessageRefusal(msg))
}
