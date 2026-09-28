package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupModelSpec_OpenAI(t *testing.T) {
	spec, ok := LookupModelSpec(ProviderOpenAI, "gpt-6.0-sol")
	require.True(t, ok)
	assert.Equal(t, "gpt-6.0-sol", spec.Name)
	assert.Equal(t, ProviderOpenAI, spec.Provider)
	assert.Equal(t, 1_050_000, spec.ContextWindow)
	assert.NotEmpty(t, spec.Pricing.Currency)

	// DefaultOpenAIModelSnapshot resolves to the same canonical spec.
	spec2, ok2 := LookupModelSpec(ProviderOpenAI, DefaultOpenAIModelSnapshot)
	require.True(t, ok2)
	assert.Equal(t, spec.Name, spec2.Name)

	_, ok3 := LookupModelSpec(ProviderOpenAI, "nonexistent")
	assert.False(t, ok3)
}

func TestLookupModelSpec_GPT60(t *testing.T) {
	tests := []struct {
		name            string
		model           string
		wantName        string
		inputPer1M      float64
		outputPer1M     float64
		cacheWritePer1M float64
		cacheReadPer1M  float64
	}{
		{
			name:            "Sol canonical model",
			model:           "gpt-6.0-sol",
			wantName:        "gpt-6.0-sol",
			inputPer1M:      2,
			outputPer1M:     10,
			cacheWritePer1M: 2.5,
			cacheReadPer1M:  0.2,
		},
		{
			name:            "Sol alias",
			model:           "gpt-6.0",
			wantName:        "gpt-6.0-sol",
			inputPer1M:      2,
			outputPer1M:     10,
			cacheWritePer1M: 2.5,
			cacheReadPer1M:  0.2,
		},
		{
			name:            "Luna canonical model",
			model:           "gpt-6.0-luna",
			wantName:        "gpt-6.0-luna",
			inputPer1M:      0.1,
			outputPer1M:     0.5,
			cacheWritePer1M: 0.125,
			cacheReadPer1M:  0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := LookupModelSpec(ProviderOpenAI, tt.model)
			require.True(t, ok)
			assert.Equal(t, tt.wantName, spec.Name)
			assert.Equal(t, 1_050_000, spec.ContextWindow)
			assert.Equal(t, []ReasoningEffort{
				ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh, ReasoningMax,
			}, spec.ReasoningEffortOptions)
			assert.InDelta(t, tt.inputPer1M, spec.Pricing.InputPer1M, 1e-9)
			assert.InDelta(t, tt.outputPer1M, spec.Pricing.OutputPer1M, 1e-9)
			assert.InDelta(t, tt.cacheWritePer1M, spec.Pricing.CacheWritePer1M, 1e-9)
			assert.InDelta(t, tt.cacheReadPer1M, spec.Pricing.CacheReadPer1M, 1e-9)
		})
	}
}

func TestDefaultModelForProvider_OpenAI(t *testing.T) {
	name, ok := DefaultModelForProvider(ProviderOpenAI)
	require.True(t, ok)
	assert.Equal(t, DefaultOpenAIModelSnapshot, name)

	_, ok2 := DefaultModelForProvider("unknown-provider")
	assert.False(t, ok2)
}

func TestDefaultModelSpecForProvider_OpenAI(t *testing.T) {
	spec, ok := DefaultModelSpecForProvider(ProviderOpenAI)
	require.True(t, ok)
	assert.Equal(t, "gpt-6.0-sol", spec.Name)
	assert.Equal(t, ProviderOpenAI, spec.Provider)

	_, ok2 := DefaultModelSpecForProvider("unknown-provider")
	assert.False(t, ok2)
}

func TestModelSpec_ToModelInfo(t *testing.T) {
	spec := SpecGPT5Mini
	info := spec.ToModelInfo("gpt-5.4-mini")
	assert.Equal(t, "gpt-5.4-mini", info.Name)
	assert.Equal(t, ProviderOpenAI, info.Provider)
	assert.Equal(t, 400_000, info.ContextWindow)
	assert.True(t, info.HasCapability(CapabilityStreaming))

	infoDisplay := spec.ToModelInfo("gpt-5.4-mini-2026-01-15")
	assert.Equal(t, "gpt-5.4-mini-2026-01-15", infoDisplay.Name)
}
