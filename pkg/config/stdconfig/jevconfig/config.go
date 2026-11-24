// Package jevconfig provides typed configuration for the platform key of the Jev classifier of
// automation graphs: the key offered to tenants as the platform credential of the connector "jev".
// It is a stdconfig package intended to be registered via config.Register[jevconfig.Config].
package jevconfig

import "fmt"

// Providers a Jev connection decides with.
const (
	ProviderOpenRouter = "openrouter"
	ProviderOpenAI     = "openai"
)

// DefaultProvider is the provider of the platform credential when Provider is empty.
const DefaultProvider = ProviderOpenRouter

// Config holds the platform key of the Jev classifier.
//
// Env prefix: "automation.jev" (e.g. AUTOMATION_JEV_APIKEY → automation.jev.apikey).
type Config struct {
	// Provider is "openrouter" or "openai" (empty: openrouter): whose key APIKey is.
	Provider string `koanf:"provider"`
	// APIKey enables the platform credential: set, the key is offered to tenants whose administrator
	// enabled it; empty, no platform credential exists.
	APIKey string `koanf:"apikey" secret:"true"`
	// DefaultModel is the exact model id written into a Jev node saved without a model; empty: the
	// provider's default (see the jev connector). An alias (latest) is refused.
	DefaultModel string `koanf:"defaultmodel"`
}

// ConfigPrefix returns the koanf prefix for jevconfig ("automation.jev").
func (Config) ConfigPrefix() string { return "automation.jev" }

// IsConfigured reports whether the platform key is set. Implements config.Configured.
func (c *Config) IsConfigured() bool { return c.APIKey != "" }

// DisabledReason explains why the platform key is off when IsConfigured returns false.
func (c *Config) DisabledReason() string { return "AUTOMATION_JEV_APIKEY required" }

// Validate checks that Provider is empty, "openrouter" or "openai".
func (c *Config) Validate() error {
	switch c.Provider {
	case "", ProviderOpenRouter, ProviderOpenAI:
		return nil
	}
	return fmt.Errorf("jevconfig: provider is %q or %q", ProviderOpenRouter, ProviderOpenAI)
}
