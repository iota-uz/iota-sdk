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
func (s ModelSpec) ToModelInfo(displayName string) ModelInfo {
	name := displayName
	if name == "" {
		name = s.Name
	}
	return ModelInfo{
		Name:          name,
		Provider:      s.Provider,
		ContextWindow: s.ContextWindow,
		Capabilities:  s.Capabilities,
	}
}

var (
	catalogMu         sync.RWMutex
	modelCatalog      = make(map[string]map[string]ModelSpec) // provider -> name -> spec
	defaultByProvider = make(map[string]string)               // provider -> default model name
)

// RegisterModelSpec registers a model spec under a provider and one or more names.
// The first name is the canonical name; additional names are aliases (e.g. versioned IDs).
// If defaultForProvider is true, this model becomes the default for the provider.
func RegisterModelSpec(provider string, names []string, spec ModelSpec, defaultForProvider bool) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if modelCatalog[provider] == nil {
		modelCatalog[provider] = make(map[string]ModelSpec)
	}
	for _, n := range names {
		modelCatalog[provider][normalizeCatalogKey(n)] = spec
	}
	if defaultForProvider {
		defaultByProvider[provider] = names[0]
	}
}

func normalizeCatalogKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// LookupModelSpec returns the ModelSpec for the given provider and model name.
// Name is normalized (lowercase, trimmed). If the exact name is not found, prefix matching
// for known families (e.g. "gpt-5.2-*") can be used by the provider when registering.
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
