package llmproviders

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// openAISpec returns the ModelSpec for this OpenAI model (catalog lookup with fallback to default).
func (m *OpenAIModel) openAISpec() agents.ModelSpec {
	if spec, ok := agents.LookupModelSpec(agents.ProviderOpenAI, m.modelName); ok {
		return spec
	}
	if spec, ok := agents.DefaultModelSpecForProvider(agents.ProviderOpenAI); ok {
		return spec
	}
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
