package controllers

import (
	"errors"
	"testing"

	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProviderStreamError(t *testing.T) {
	t.Parallel()

	raw := `StreamController.StreamMessage: Executor.Execute: OpenAIModel.Stream: stream error: received error while streaming: {"type":"insufficient_quota","code":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details.","param":null}`
	code, message, ok := parseProviderStreamError(raw)
	require.True(t, ok)
	assert.Equal(t, "insufficient_quota", code)
	assert.Equal(t, "You exceeded your current quota, please check your plan and billing details.", message)
}

func TestStreamClientErrorMessage_KnownProviderError(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New(`OpenAIModel.Stream: stream error: {"type":"insufficient_quota","code":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details."}`)
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Contains(t, got, "You exceeded your current quota")
}

func TestStreamClientErrorMessage_UnknownProviderErrorReturnsGeneric(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New("Executor.Execute: internal failure")
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, "An error occurred while processing your request", got)
}
