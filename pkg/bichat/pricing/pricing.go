// Package pricing provides shared BiChat model pricing lookup and cost calculation.
package pricing

import (
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/sirupsen/logrus"
)

const overridesEnv = "BICHAT_MODEL_PRICING_OVERRIDES_JSON"

type ModelPricing struct {
	InputPerMTok      float64 `json:"inputPerMTok"`
	OutputPerMTok     float64 `json:"outputPerMTok"`
	CacheWritePerMTok float64 `json:"cacheWritePerMTok"`
	CacheReadPerMTok  float64 `json:"cacheReadPerMTok"`
}

type Breakdown struct {
	Input      float64
	Output     float64
	CacheWrite float64
	CacheRead  float64
	Total      float64
}

type Registry struct {
	entries map[string]ModelPricing
}

type overrideModelPricing struct {
	InputPerMTok      float64 `json:"inputPerMTok"`
	OutputPerMTok     float64 `json:"outputPerMTok"`
	CacheWritePerMTok float64 `json:"cacheWritePerMTok"`
	CacheReadPerMTok  float64 `json:"cacheReadPerMTok"`
	InputPer1M        float64 `json:"inputPer1M"`
	OutputPer1M       float64 `json:"outputPer1M"`
	CacheWritePer1M   float64 `json:"cacheWritePer1M"`
	CacheReadPer1M    float64 `json:"cacheReadPer1M"`
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
	warnedUnknown       sync.Map
)

func Default() *Registry {
	defaultRegistryOnce.Do(func() {
		defaultRegistry = newRegistry(strings.TrimSpace(lookupEnv(overridesEnv)))
	})
	return defaultRegistry
}

func Compute(modelID string, usage types.TokenUsage) Breakdown {
	return Default().Compute(modelID, usage)
}

func (r *Registry) Compute(modelID string, usage types.TokenUsage) Breakdown {
	pricing, ok := r.lookup(modelID)
	if !ok {
		warnUnknownModel(modelID)
		return Breakdown{}
	}

	inputCost := (float64(usage.PromptTokens) / 1_000_000) * pricing.InputPerMTok
	outputCost := (float64(usage.CompletionTokens) / 1_000_000) * pricing.OutputPerMTok
	cacheWriteCost := (float64(usage.CacheWriteTokens) / 1_000_000) * pricing.CacheWritePerMTok
	cacheReadCost := (float64(usage.CacheReadTokens) / 1_000_000) * pricing.CacheReadPerMTok

	return Breakdown{
		Input:      inputCost,
		Output:     outputCost,
		CacheWrite: cacheWriteCost,
		CacheRead:  cacheReadCost,
		Total:      inputCost + outputCost + cacheWriteCost + cacheReadCost,
	}
}

func (r *Registry) lookup(modelID string) (ModelPricing, bool) {
	if r == nil {
		return ModelPricing{}, false
	}

	pricing, ok := r.entries[normalizeKey(modelID)]
	return pricing, ok
}

func newRegistry(overridesJSON string) *Registry {
	entries := defaultEntries()
	mergeOverrides(entries, overridesJSON)
	return &Registry{entries: entries}
}

func defaultEntries() map[string]ModelPricing {
	entries := map[string]ModelPricing{
		normalizeKey("gpt-5.4-2026-03-05"): {
			InputPerMTok:     2.50,
			OutputPerMTok:    15.00,
			CacheReadPerMTok: 0.25,
		},
		normalizeKey("gpt-5.4"): {
			InputPerMTok:     2.50,
			OutputPerMTok:    15.00,
			CacheReadPerMTok: 0.25,
		},
		normalizeKey("gpt-5.2"): {
			InputPerMTok:     1.75,
			OutputPerMTok:    14.00,
			CacheReadPerMTok: 0.175,
		},
		normalizeKey("gpt-5.2-2025-12-11"): {
			InputPerMTok:     1.75,
			OutputPerMTok:    14.00,
			CacheReadPerMTok: 0.175,
		},
		normalizeKey("gpt-5.4-mini"): {
			InputPerMTok:     0.25,
			OutputPerMTok:    2.00,
			CacheReadPerMTok: 0.025,
		},
		normalizeKey("gpt-5-mini"): {
			InputPerMTok:     0.25,
			OutputPerMTok:    2.00,
			CacheReadPerMTok: 0.025,
		},
		normalizeKey("gpt-5.4-nano"): {
			InputPerMTok:     0.05,
			OutputPerMTok:    0.40,
			CacheReadPerMTok: 0.005,
		},
		normalizeKey("gpt-5-nano"): {
			InputPerMTok:     0.05,
			OutputPerMTok:    0.40,
			CacheReadPerMTok: 0.005,
		},
	}

	entries[normalizeKey("claude-sonnet-4-6")] = ModelPricing{
		InputPerMTok:      3.00,
		OutputPerMTok:     15.00,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.30,
	}
	entries[normalizeKey("claude-sonnet-4")] = entries[normalizeKey("claude-sonnet-4-6")]
	entries[normalizeKey("claude-opus-4-6")] = ModelPricing{
		InputPerMTok:      15.00,
		OutputPerMTok:     75.00,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.50,
	}
	entries[normalizeKey("claude-opus-4")] = entries[normalizeKey("claude-opus-4-6")]

	return entries
}

func mergeOverrides(entries map[string]ModelPricing, overridesJSON string) {
	if strings.TrimSpace(overridesJSON) == "" {
		return
	}

	var overrides map[string]overrideModelPricing
	if err := json.Unmarshal([]byte(overridesJSON), &overrides); err != nil {
		logrus.WithError(err).Warn("bichat pricing: failed to parse pricing overrides")
		return
	}

	for modelID, override := range overrides {
		entries[normalizeKey(modelID)] = ModelPricing{
			InputPerMTok:      firstNonZero(override.InputPerMTok, override.InputPer1M),
			OutputPerMTok:     firstNonZero(override.OutputPerMTok, override.OutputPer1M),
			CacheWritePerMTok: firstNonZero(override.CacheWritePerMTok, override.CacheWritePer1M),
			CacheReadPerMTok:  firstNonZero(override.CacheReadPerMTok, override.CacheReadPer1M),
		}
	}
}

func warnUnknownModel(modelID string) {
	normalized := normalizeKey(modelID)
	if normalized == "" {
		return
	}
	if _, loaded := warnedUnknown.LoadOrStore(normalized, struct{}{}); loaded {
		return
	}
	logrus.WithField("model", modelID).Warn("bichat pricing: no pricing configured for model")
}

func normalizeKey(modelID string) string {
	return strings.ToLower(strings.TrimSpace(modelID))
}

func firstNonZero(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

var lookupEnv = func(key string) string {
	return strings.TrimSpace(getenv(key))
}

var getenv = func(key string) string {
	return os.Getenv(key)
}
