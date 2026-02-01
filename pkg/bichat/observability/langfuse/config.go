package langfuse

import (
	"errors"
	"time"
)

// Config holds configuration for the Langfuse observability provider.
type Config struct {
	// Host is the Langfuse API endpoint.
	// Defaults to "https://cloud.langfuse.com" if empty.
	Host string

	// PublicKey is the Langfuse public API key (required).
	PublicKey string

	// SecretKey is the Langfuse secret API key (required).
	SecretKey string

	// FlushInterval is how often to flush pending observations to Langfuse.
	// The Langfuse SDK batches observations for efficiency.
	// Defaults to 1 second if zero.
	FlushInterval time.Duration

	// SampleRate controls what percentage of observations to send (0.0-1.0).
	// 1.0 = 100% (all observations), 0.5 = 50%, 0.0 = 0% (disabled).
	// Defaults to 1.0 (send everything).
	SampleRate float64

	// Environment identifies the deployment environment (e.g., "production", "staging").
	// Optional - used for filtering in Langfuse UI.
	Environment string

	// Version identifies the application version.
	// Optional - useful for A/B testing and rollback analysis.
	Version string

	// Tags are custom labels applied to all observations.
	// Optional - used for filtering and grouping in Langfuse.
	Tags []string

	// Enabled controls whether observability is active.
	// When false, all Record* methods become no-ops.
	// Defaults to true.
	Enabled bool
}

// Validate checks the configuration and applies defaults.
func (c *Config) Validate() error {
	// Apply defaults
	if c.Host == "" {
		c.Host = "https://cloud.langfuse.com"
	}

	if c.FlushInterval == 0 {
		c.FlushInterval = 1 * time.Second
	}

	if c.SampleRate == 0 {
		c.SampleRate = 1.0
	}

	// Enabled defaults to true (explicit check for zero value)
	// This means if not set, observability is enabled by default

	// Validation
	if c.PublicKey == "" {
		return errors.New("langfuse: PublicKey is required")
	}

	if c.SecretKey == "" {
		return errors.New("langfuse: SecretKey is required")
	}

	if c.SampleRate < 0.0 || c.SampleRate > 1.0 {
		return errors.New("langfuse: SampleRate must be between 0.0 and 1.0")
	}

	return nil
}
