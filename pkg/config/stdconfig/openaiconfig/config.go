// Package openaiconfig provides typed configuration for the platform's OpenAI key of automation graphs:
// the key offered to tenants as the platform credential of the connector "openai" (the LLM node).
// It is a stdconfig package intended to be registered via config.Register[openaiconfig.Config].
//
// It is not the BiChat key (bichat.openai): AI spending of automations is accounted separately.
package openaiconfig

import (
	"fmt"
	"net/url"
	"strings"
)

// DefaultBaseURL is the API the connector uses when BaseURL is empty.
const DefaultBaseURL = "https://api.openai.com/v1"

// DefaultModel is the pinned snapshot an LLM node without a model gets when DefaultModel is empty.
const DefaultModel = "gpt-4o-mini-2024-07-18"

// Config holds the platform OpenAI key of automations.
//
// Env prefix: "automation.openai" (e.g. AUTOMATION_OPENAI_APIKEY → automation.openai.apikey).
type Config struct {
	// APIKey enables the platform credential: set, the key is offered to tenants whose administrator
	// enabled it; empty, no platform credential exists.
	APIKey string `koanf:"apikey" secret:"true"`
	// BaseURL is an OpenAI-compatible endpoint (https only); empty: DefaultBaseURL.
	BaseURL string `koanf:"baseurl"`
	// DefaultModel is the exact model id written into an LLM node saved without a model; empty:
	// DefaultModel. An alias (latest) or a rolling id (gpt-4o-mini without a date) is refused.
	DefaultModel string `koanf:"defaultmodel"`
}

// ConfigPrefix returns the koanf prefix for openaiconfig ("automation.openai").
func (Config) ConfigPrefix() string { return "automation.openai" }

// IsConfigured reports whether the platform key is set. Implements config.Configured; there is no
// separate enable flag.
func (c *Config) IsConfigured() bool { return c.APIKey != "" }

// DisabledReason explains why the platform key is off when IsConfigured returns false.
func (c *Config) DisabledReason() string { return "AUTOMATION_OPENAI_APIKEY required" }

// Validate checks that BaseURL, when set, is an https URL: the key is sent to it.
func (c *Config) Validate() error {
	raw := strings.TrimSpace(c.BaseURL)
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return fmt.Errorf("openaiconfig: baseurl is an https URL")
	}
	return nil
}
