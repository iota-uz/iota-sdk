package agents

import (
	"strings"
	"sync"
)

// ModelSpec is the canonical metadata for a model (pricing, context window, capabilities).
// Providers resolve model names to a spec and expose it via Model.Info() and Model.Pricing().
type ModelSpec struct {
	Name          string
	Provider      string
	ContextWindow int
	Capabilities  []Capability
	Pricing       ModelPricing
}

// ToModelInfo returns ModelInfo for this spec, using the given display name.
// Use the requested model name (e.g. "gpt-5.2-2025-12-11") so observability shows the actual model used.
// Capabilities are copied so the returned struct is safe from mutation by callers.
func (s ModelSpec) ToModelInfo(displayName string) ModelInfo {
	name := displayName
	if name == "" {
		name = s.Name
	}
	caps := make([]Capability, len(s.Capabilities))
	copy(caps, s.Capabilities)
	return ModelInfo{
		Name:          name,
		Provider:      s.Provider,
		ContextWindow: s.ContextWindow,
		Capabilities:  caps,
	}
}

var (
	catalogMu         sync.RWMutex
	modelCatalog      = make(map[string]map[string]ModelSpec) // provider -> name -> spec
	defaultByProvider = make(map[string]string)               // provider -> default model name
)

// RegisterModelSpec registers a model spec under a provider and one or more names.
// The first non-blank name is the canonical name; additional names are aliases (e.g. versioned IDs).
// If defaultForProvider is true, this model becomes the default for the provider (using first non-blank name).
// Empty provider or nil/empty names are ignored.
func RegisterModelSpec(provider string, names []string, spec ModelSpec, defaultForProvider bool) {
	provider = strings.TrimSpace(provider)
	if provider == "" || len(names) == 0 {
		return
	}
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if modelCatalog[provider] == nil {
		modelCatalog[provider] = make(map[string]ModelSpec)
	}
	var firstKey string
	for _, n := range names {
		key := normalizeCatalogKey(n)
		if key == "" {
			continue
		}
		if firstKey == "" {
			firstKey = key
		}
		modelCatalog[provider][key] = spec
	}
	if defaultForProvider && firstKey != "" {
		defaultByProvider[provider] = firstKey
	}
}

func normalizeCatalogKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// LookupModelSpec returns the ModelSpec for the given provider and model name.
// The name is normalized (lowercase, trimmed); registration uses the same normalization.
// Returns (spec, true) if found, (ModelSpec{}, false) otherwise.
func LookupModelSpec(provider, modelName string) (ModelSpec, bool) {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	key := normalizeCatalogKey(modelName)
	byName, ok := modelCatalog[provider]
	if !ok {
		return ModelSpec{}, false
	}
	if spec, ok := byName[key]; ok {
		return spec, true
	}
	return ModelSpec{}, false
}

// DefaultModelForProvider returns the default model name for the provider (e.g. "gpt-5.2" for "openai").
// Returns ("", false) if the provider has no default.
func DefaultModelForProvider(provider string) (string, bool) {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	name, ok := defaultByProvider[provider]
	return name, ok
}

// DefaultModelSpecForProvider returns the default model spec for the provider.
// Returns (spec, true) if the provider has a default and it is registered; (ModelSpec{}, false) otherwise.
func DefaultModelSpecForProvider(provider string) (ModelSpec, bool) {
	name, ok := DefaultModelForProvider(provider)
	if !ok {
		return ModelSpec{}, false
	}
	return LookupModelSpec(provider, name)
}
