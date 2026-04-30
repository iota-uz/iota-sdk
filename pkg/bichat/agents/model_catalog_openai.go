// Package agents provides this package.
package agents

// OpenAI model specs. Context and pricing from platform.openai.com/docs/models.

// ProviderOpenAI is the provider identifier for OpenAI; use it with LookupModelSpec and DefaultModelForProvider.
const ProviderOpenAI = "openai"

// DefaultOpenAIModelSnapshot is the pinned OpenAI default. Currently the bare
// alias "gpt-5.5" — replace with the dated snapshot (e.g. "gpt-5.5-YYYY-MM-DD")
// once OpenAI publishes one to immunize against floating-alias rollovers.
const DefaultOpenAIModelSnapshot = "gpt-5.5"

var (
	// SpecGPT55 is the canonical spec for GPT-5.5.
	//
	// Pricing per platform.openai.com/docs/models (Input $5.00 / Cached input
	// $0.50 / Output $30.00 per 1M tokens). Context window inherited from
	// GPT-5.4 baseline (1.05M) until OpenAI publishes a divergent value;
	// adjust if the docs disagree.
	SpecGPT55 = ModelSpec{
		Name:          "gpt-5.5",
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
			InputPer1M:      5.00,
			OutputPer1M:     30.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.50,
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
	//
	// Pricing per platform.openai.com/docs/models (Input $0.75 / Cached input
	// $0.075 / Output $4.50 per 1M tokens).
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
		ReasoningEffortOptions: []ReasoningEffort{
			ReasoningLow, ReasoningMedium, ReasoningHigh,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.75,
			OutputPer1M:     4.50,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.075,
		},
	}

	// SpecGPT5Nano is the spec for GPT-5.4 nano (400K context).
	//
	// Pricing per platform.openai.com/docs/models (Input $0.20 / Cached input
	// $0.02 / Output $1.25 per 1M tokens).
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
			InputPer1M:      0.20,
			OutputPer1M:     1.25,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0.02,
		},
	}
)

func init() {
	// GPT-5.5 is the provider default. DefaultOpenAIModelSnapshot is the
	// canonical name; add dated snapshots ("gpt-5.5-YYYY-MM-DD") to the alias
	// list once OpenAI publishes them.
	RegisterModelSpec(ProviderOpenAI, []string{DefaultOpenAIModelSnapshot}, SpecGPT55, true)

	// GPT-5.2: canonical name + versioned alias
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.2", "gpt-5.2-2025-12-11"}, SpecGPT52, false)

	// Mini and nano: register the canonical name plus its bare-major alias.
	// "gpt-5-mini" / "gpt-5-nano" are OpenAI's floating-major aliases that
	// previously fell through to the frontier spec by accident — silently
	// charging mini/nano prices for full-spec context windows. Treating them
	// as proper aliases keeps both context+caps and pricing aligned.
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.4-mini", "gpt-5-mini"}, SpecGPT5Mini, false)
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.4-nano", "gpt-5-nano"}, SpecGPT5Nano, false)
}
