package controllers

import (
	"errors"
	"testing"

	bichatmodsvcs "github.com/iota-uz/iota-sdk/modules/bichat/services"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProviderStreamError(t *testing.T) {
	t.Parallel()

	raw := `StreamController.StreamMessage: Executor.Execute: OpenAIModel.Stream: stream error: received error while streaming: {"type":"insufficient_quota","code":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details.","param":null}`
	code, message, ok := bichatmodsvcs.ParseProviderStreamError(raw)
	require.True(t, ok)
	assert.Equal(t, "insufficient_quota", code)
	assert.Equal(t, "You exceeded your current quota, please check your plan and billing details.", message)
}

func TestParseProviderStreamError_NoJSON(t *testing.T) {
	t.Parallel()

	code, message, ok := bichatmodsvcs.ParseProviderStreamError("plain upstream failure")
	assert.False(t, ok)
	assert.Empty(t, code)
	assert.Empty(t, message)
}

func TestParseProviderStreamError_CodeMarker(t *testing.T) {
	t.Parallel()

	raw := `provider stream failed: {"code":"rate_limit_exceeded","message":"Rate limit exceeded.","type":""}`
	code, message, ok := bichatmodsvcs.ParseProviderStreamError(raw)
	require.True(t, ok)
	assert.Equal(t, "rate_limit_exceeded", code)
	assert.Equal(t, "Rate limit exceeded.", message)
}

func TestStreamClientErrorMessage_KnownProviderError(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New(`OpenAIModel.Stream: stream error: {"type":"insufficient_quota","code":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details."}`)
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, bichatmodsvcs.ProviderBillingQuotaError, got)
}

func TestStreamClientErrorMessage_CreditBalanceExhausted(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New(`OpenAIModel.Stream: stream error: {"type":"insufficient_quota","code":"credit_balance_exhausted","message":"Your credit balance is too low."}`)
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, bichatmodsvcs.ProviderBillingBalanceExhaustedError, got)
}

func TestStreamClientErrorMessage_NilError(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	assert.Empty(t, controller.streamClientErrorMessage(nil, bichatservices.ChunkTypeError))
}

func TestStreamClientErrorMessage_NonErrorChunkTypeIsSanitized(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New("internal stack trace with sensitive details")
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkType("content_like"))
	assert.Equal(t, "internal error", got)
}

func TestStreamClientErrorMessage_KnownRateLimitCode(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New(`provider stream error: {"code":"rate_limit_exceeded","message":"Too many requests."}`)
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, bichatmodsvcs.ProviderRateLimitedError, got)
}

func TestStreamClientErrorMessage_KnownInvalidAPIKeyCode(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New(`provider stream error: {"code":"invalid_api_key","message":"Invalid key."}`)
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, bichatmodsvcs.ProviderAuthInvalidError, got)
}

func TestStreamClientErrorMessage_UnknownProviderErrorReturnsGeneric(t *testing.T) {
	t.Parallel()

	controller := &StreamController{}
	err := errors.New("Executor.Execute: internal failure")
	got := controller.streamClientErrorMessage(err, bichatservices.ChunkTypeError)
	assert.Equal(t, "An error occurred while processing your request", got)
}
