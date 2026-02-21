package llmproviders

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

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

func contextWindowForModel(modelName string) int {
	normalizedModelName := strings.ToLower(strings.TrimSpace(modelName))

	if strings.HasPrefix(normalizedModelName, "gpt-5.2") {
		return 272000
	}

	modelContextWindows := map[string]int{
		"gpt-4o":      128000,
		"gpt-4o-mini": 128000,
		"gpt-4-turbo": 128000,
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
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	pricing := map[string]agents.ModelPricing{
		"gpt-5.2-2025-12-11": {
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.18,
		},
		"gpt-4o": {
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 1.25,
			CacheReadPer1M:  0.625,
		},
		"gpt-4o-mini": {
			Currency:        "USD",
			InputPer1M:      0.150,
			OutputPer1M:     0.600,
			CacheWritePer1M: 0.075,
			CacheReadPer1M:  0.038,
		},
		"gpt-4-turbo": {
			Currency:        "USD",
			InputPer1M:      10.00,
			OutputPer1M:     30.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		},
		"gpt-4": {
			Currency:        "USD",
			InputPer1M:      30.00,
			OutputPer1M:     60.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
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
		CacheReadPer1M:  0.18,
	}
}
