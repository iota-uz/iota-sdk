// Package agents provides this package.
package agents

// OpenAI model specs. Context and pricing from platform.openai.com/docs/models.

// ProviderOpenAI is the provider identifier for OpenAI; use it with LookupModelSpec and DefaultModelForProvider.
const ProviderOpenAI = "openai"

// DefaultOpenAIModelSnapshot is the pinned OpenAI default. Currently
// "gpt-6.0-sol" — replace with a dated snapshot (e.g. "gpt-6.0-YYYY-MM-DD")
// once OpenAI publishes one to immunize against floating-alias rollovers.
const DefaultOpenAIModelSnapshot = "gpt-6.0-sol"

var (
	// SpecGPT60Sol is the canonical spec for GPT-6.0 Sol.
	//
	// Pricing per developers.openai.com/api/docs/pricing, standard tier
	// (Input $2.00 / Cached input $0.20 / Cache writes $2.50 /
	// Output $10.00 per 1M tokens).
	SpecGPT60Sol = ModelSpec{
		Name:          "gpt-6.0-sol",
		Provider:      ProviderOpenAI,
		ContextWindow: 1_050_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		ReasoningEffortOptions: []ReasoningEffort{
			ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh, ReasoningMax,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      2.00,
			OutputPer1M:     10.00,
			CacheWritePer1M: 2.50,
			CacheReadPer1M:  0.20,
		},
	}

	// SpecGPT60Luna is the canonical spec for GPT-6.0 Luna.
	//
	// Pricing per developers.openai.com/api/docs/pricing, standard tier
	// (Input $0.10 / Cached input $0.01 / Cache writes $0.125 /
	// Output $0.50 per 1M tokens).
	SpecGPT60Luna = ModelSpec{
		Name:          "gpt-6.0-luna",
		Provider:      ProviderOpenAI,
		ContextWindow: 1_050_000,
		Capabilities: []Capability{
			CapabilityStreaming,
			CapabilityTools,
			CapabilityJSONMode,
			CapabilityThinking,
		},
		ReasoningEffortOptions: []ReasoningEffort{
			ReasoningNone, ReasoningLow, ReasoningMedium, ReasoningHigh, ReasoningXHigh, ReasoningMax,
		},
		Pricing: ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.10,
			OutputPer1M:     0.50,
			CacheWritePer1M: 0.125,
			CacheReadPer1M:  0.01,
		},
	}

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
	// GPT-6.0 Sol is the flagship model; "gpt-6.0" is its floating alias.
	// It is also the provider default; DefaultOpenAIModelSnapshot is the
	// canonical name. Add dated snapshots ("gpt-6.0-YYYY-MM-DD") to the alias
	// list once OpenAI publishes them.
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-6.0-sol", "gpt-6.0"}, SpecGPT60Sol, true)

	// GPT-6.0 Luna is the cost-sensitive model in the GPT-6.0 family.
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-6.0-luna"}, SpecGPT60Luna, false)

	// GPT-5.5: canonical name; no longer the provider default. Add dated
	// snapshots ("gpt-5.5-YYYY-MM-DD") to the alias list once OpenAI
	// publishes them.
	RegisterModelSpec(ProviderOpenAI, []string{"gpt-5.5"}, SpecGPT55, false)

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
