package llmproviders

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// Model context windows and pricing are kept in sync with official docs:
// - OpenAI: platform.openai.com/docs/models (GPT-5.2: 400K ctx, $1.75/$14, cache read $0.175; mini/nano from model pages).
// - Claude: docs.anthropic.com (200K standard, 1M with beta; pricing in cost tests: Sonnet 4.6 $3/$15, Opus 4.6 $5/$25, Haiku 4.5 $1/$5).

// Info returns model metadata including capabilities.
func (m *OpenAIModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:          m.modelName,
		Provider:      "openai",
		ContextWindow: contextWindowForModel(m.modelName),
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityJSONMode,
		},
	}
}

// contextWindowForModel returns context window size per official docs (OpenAI: platform.openai.com/docs/models).
// GPT-5.2: 400K; GPT-5 mini/nano: 400K.
func contextWindowForModel(modelName string) int {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))

	if strings.HasPrefix(normalizedModelName, "gpt-5.2") {
		return 400000
	}
	if strings.HasPrefix(normalizedModelName, "gpt-5-mini") {
		return 400000
	}
	if strings.HasPrefix(normalizedModelName, "gpt-5-nano") {
		return 400000
	}

	modelContextWindows := map[string]int{
		"gpt-5-mini": 400000,
		"gpt-5-nano": 400000,
	}

	if contextWindow, ok := modelContextWindows[normalizedModelName]; ok {
		return contextWindow
	}

	return 0
}

// HasCapability checks if this model supports a specific capability.
func (m *OpenAIModel) HasCapability(capability agents.Capability) bool {
	return m.Info().HasCapability(capability)
}

// ModelParameters returns the model-level parameters sent to the API.
// Implements agents.ModelParameterReporter for observability.
func (m *OpenAIModel) ModelParameters() map[string]interface{} {
	return map[string]interface{}{
		"store": true,
	}
}

// Pricing returns pricing information for this OpenAI model.
// Rates per 1M tokens from platform.openai.com (GPT-5.2); mini/nano from official model pages.
// Cache: GPT-5.2 cached input = 0.175 (90% discount); mini/nano cached rates applied where documented.
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	pricing := map[string]agents.ModelPricing{
		"gpt-5.2": {
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.175,
		},
		"gpt-5.2-2025-12-11": {
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.175,
		},
		"gpt-5-mini": {
			Currency:        "USD",
			InputPer1M:      0.25,
			OutputPer1M:     2.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.025,
		},
		"gpt-5-nano": {
			Currency:        "USD",
			InputPer1M:      0.05,
			OutputPer1M:     0.40,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.005,
		},
	}

	if p, exists := pricing[m.modelName]; exists {
		return p
	}

	return agents.ModelPricing{
		Currency:        "USD",
		InputPer1M:      1.75,
		OutputPer1M:     14.00,
		CacheWritePer1M: 0,
		CacheReadPer1M:  0.175,
	}
}
