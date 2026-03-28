// Package agents provides this package.
package agents

// OpenAI model specs. Context and pricing from platform.openai.com/docs/models.

// ProviderOpenAI is the provider identifier for OpenAI; use it with LookupModelSpec and DefaultModelForProvider.
const ProviderOpenAI = "openai"

// DefaultOpenAIModelSnapshot is the pinned OpenAI snapshot used as the provider default.
const DefaultOpenAIModelSnapshot = "gpt-5.4-2026-03-05"

var (
	// SpecGPT54 is the canonical spec for GPT-5.4.
	//
	// Context window, capability support, and pricing are based on the official OpenAI model docs.
	SpecGPT54 = ModelSpec{
		Name:          "gpt-5.4",
		Provider:      ProviderOpenAI,
		ContextWindow: 1_050_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		ReasoningEffortOptions: []ReasoningEffort{
			ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     15.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.25,
		},
	}

	// SpecGPT52 is the canonical spec for GPT-5.2 (400K context, cache read discount).
	SpecGPT52 = ModelSpec{
		Name:          "gpt-5.2",
		Provider:      ProviderOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		ReasoningEffortOptions: []ReasoningEffort{
			ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.175,
		},
	}

	// SpecGPT5Mini is the spec for GPT-5.4 mini (400K context).
	SpecGPT5Mini = ModelSpec{
		Name:          "gpt-5.4-mini",
		Provider:      ProviderOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.25,
			OutputPer1M:     2.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.025,
		},
	}

	// SpecGPT5Nano is the spec for GPT-5.4 nano (400K context).
	SpecGPT5Nano = ModelSpec{
		Name:          "gpt-5.4-nano",
		Provider:      ProviderOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.05,
			OutputPer1M:     0.40,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.005,
		},
	}
)

func init() {
	// GPT-5.4: snapshot alias first so provider default resolves to the requested snapshot.
	RegisterModelSpec(ProviderOpenAI, []string{DefaultOpenAIModelSnapshot, "gpt-5.4"}, SpecGPT54, true)

	// GPT-5.2: canonical name + versioned alias
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.2", "gpt-5.2-2025-12-11"}, SpecGPT52, false)

	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.4-mini"}, SpecGPT5Mini, false)
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.4-nano"}, SpecGPT5Nano, false)
}
