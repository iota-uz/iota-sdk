package llmproviders

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

const providerOpenAI = "openai"

// resolveOpenAIModelKey maps a requested model name to a catalog key for lookup.
// Exact matches are used as-is; versioned names (e.g. gpt-5.2-2025-12-11) are resolved
// by prefix to the canonical catalog key so they share the same spec.
func resolveOpenAIModelKey(modelName string) string {
	n := strings.ToLower(strings.TrimSpace(modelName))
	if n == "" {
		return n
	}
	switch {
	case strings.HasPrefix(n, "gpt-5.2"):
		return "gpt-5.2"
	case strings.HasPrefix(n, "gpt-5-mini"):
		return "gpt-5-mini"
	case strings.HasPrefix(n, "gpt-5-nano"):
		return "gpt-5-nano"
	default:
		return n
	}
}

// openAISpec returns the ModelSpec for this OpenAI model (catalog lookup with fallback to default).
func (m *OpenAIModel) openAISpec() agents.ModelSpec {
	key := resolveOpenAIModelKey(m.modelName)
	if spec, ok := agents.LookupModelSpec(providerOpenAI, key); ok {
		return spec
	}
	// Unknown model: use provider default spec if available
	if defaultName, ok := agents.DefaultModelForProvider(providerOpenAI); ok {
		if spec, ok := agents.LookupModelSpec(providerOpenAI, defaultName); ok {
			return spec
		}
	}
	// Last resort: return GPT-5.2 spec so callers get valid info/pricing
	return agents.SpecGPT52
}

// Info returns model metadata including capabilities from the shared catalog.
func (m *OpenAIModel) Info() agents.ModelInfo {
	spec := m.openAISpec()
	return spec.ToModelInfo(m.modelName)
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

// Pricing returns pricing information for this OpenAI model from the shared catalog.
func (m *OpenAIModel) Pricing() agents.ModelPricing {
	return m.openAISpec().Pricing
}
