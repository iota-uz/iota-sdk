package pricing

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
)

func TestRegistryCompute_KnownModel(t *testing.T) {
	t.Parallel()

	registry := newRegistry("")
	breakdown := registry.Compute("claude-sonnet-4-6", types.TokenUsage{
		PromptTokens:     1_000_000,
		CompletionTokens: 500_000,
		CacheWriteTokens: 250_000,
		CacheReadTokens:  100_000,
	})

	assert.InDelta(t, 3.0, breakdown.Input, 1e-9)
	assert.InDelta(t, 7.5, breakdown.Output, 1e-9)
	assert.InDelta(t, 0.9375, breakdown.CacheWrite, 1e-9)
	assert.InDelta(t, 0.03, breakdown.CacheRead, 1e-9)
	assert.InDelta(t, 11.4675, breakdown.Total, 1e-9)
}

func TestRegistryCompute_Override(t *testing.T) {
	t.Parallel()

	registry := newRegistry(`{"claude-sonnet-4-6":{"inputPerMTok":10,"outputPerMTok":20}}`)
	breakdown := registry.Compute("claude-sonnet-4-6", types.TokenUsage{
		PromptTokens:     100_000,
		CompletionTokens: 50_000,
	})

	assert.InDelta(t, 1.0, breakdown.Input, 1e-9)
	assert.InDelta(t, 1.0, breakdown.Output, 1e-9)
	assert.InDelta(t, 2.0, breakdown.Total, 1e-9)
}

func TestRegistryCompute_UnknownModel(t *testing.T) {
	t.Parallel()

	registry := newRegistry("")
	breakdown := registry.Compute("mystery-model", types.TokenUsage{
		PromptTokens:     100,
		CompletionTokens: 100,
	})

	assert.Zero(t, breakdown.Input)
	assert.Zero(t, breakdown.Output)
	assert.Zero(t, breakdown.Total)
}
