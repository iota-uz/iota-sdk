package agents

// OpenAI model specs. Context and pricing from platform.openai.com/docs/models.
const providerOpenAI = "openai"

var (
	// SpecGPT52 is the canonical spec for GPT-5.2 (400K context, cache read discount).
	SpecGPT52 = ModelSpec{
		Name:          "gpt-5.2",
		Provider:      providerOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      1.75,
			OutputPer1M:     14.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.175,
		},
	}

	// SpecGPT5Mini is the spec for GPT-5 mini (400K context).
	SpecGPT5Mini = ModelSpec{
		Name:          "gpt-5-mini",
		Provider:      providerOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.25,
			OutputPer1M:     2.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.025,
		},
	}

	// SpecGPT5Nano is the spec for GPT-5 nano (400K context).
	SpecGPT5Nano = ModelSpec{
		Name:          "gpt-5-nano",
		Provider:      providerOpenAI,
		ContextWindow: 400_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
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
	// GPT-5.2: canonical name + versioned alias
	RegisterModelSpec(providerOpenAI, []string{"gpt-5.2", "gpt-5.2-2025-12-11"}, SpecGPT52, true)

	RegisterModelSpec(providerOpenAI, []string{"gpt-5-mini"}, SpecGPT5Mini, false)
	RegisterModelSpec(providerOpenAI, []string{"gpt-5-nano"}, SpecGPT5Nano, false)
}
