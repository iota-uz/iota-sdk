package agents

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupModelSpec_OpenAI(t *testing.T) {
	spec, ok := LookupModelSpec(ProviderOpenAI, "gpt-5.2")
	require.True(t, ok)
	assert.Equal(t, "gpt-5.2", spec.Name)
	assert.Equal(t, ProviderOpenAI, spec.Provider)
	assert.Equal(t, 400_000, spec.ContextWindow)
	assert.NotEmpty(t, spec.Pricing.Currency)

	// Versioned alias resolves to same canonical spec
	spec2, ok2 := LookupModelSpec(ProviderOpenAI, "gpt-5.2-2025-12-11")
	require.True(t, ok2)
	assert.Equal(t, spec.Name, spec2.Name)

	_, ok3 := LookupModelSpec(ProviderOpenAI, "nonexistent")
	assert.False(t, ok3)
}

func TestDefaultModelForProvider_OpenAI(t *testing.T) {
	name, ok := DefaultModelForProvider(ProviderOpenAI)
	require.True(t, ok)
	assert.Equal(t, "gpt-5.2", name)

	_, ok2 := DefaultModelForProvider("unknown-provider")
	assert.False(t, ok2)
}

func TestDefaultModelSpecForProvider_OpenAI(t *testing.T) {
	spec, ok := DefaultModelSpecForProvider(ProviderOpenAI)
	require.True(t, ok)
	assert.Equal(t, "gpt-5.2", spec.Name)
	assert.Equal(t, ProviderOpenAI, spec.Provider)

	_, ok2 := DefaultModelSpecForProvider("unknown-provider")
	assert.False(t, ok2)
}

func TestModelSpec_ToModelInfo(t *testing.T) {
	spec := SpecGPT5Mini
	info := spec.ToModelInfo("gpt-5-mini")
	assert.Equal(t, "gpt-5-mini", info.Name)
	assert.Equal(t, ProviderOpenAI, info.Provider)
	assert.Equal(t, 400_000, info.ContextWindow)
	assert.True(t, info.HasCapability(CapabilityStreaming))

	infoDisplay := spec.ToModelInfo("gpt-5-mini-2026-01-15")
	assert.Equal(t, "gpt-5-mini-2026-01-15", infoDisplay.Name)
}
