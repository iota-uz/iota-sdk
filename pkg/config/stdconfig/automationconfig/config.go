// Package automationconfig provides typed configuration for automation graphs: the keyring that
// encrypts connector credentials in automation.connections.
// Intended to be registered via config.Register[automationconfig.Config].
package automationconfig

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

// ConnectionsConfig configures the connection store.
type ConnectionsConfig struct {
	// Keys is the keyring "id:key[,id:key...]": each key 32 bytes in base64 or 64 hex digits, the first
	// one active. Empty: tenant connections are unavailable; platform credentials still work. To rotate,
	// put the new key first, keep the old ones until Rewrap has finished, then drop them.
	Keys string `koanf:"keys" secret:"true"`
	// Active names the key that encrypts (empty: the first). With several replicas, roll a new key out
	// decrypt-only first (listed, not active), make it active in a second deploy, then run Rewrap.
	Active string `koanf:"active"`
}

// Config holds automation settings.
// Env prefix: "automation" (e.g. AUTOMATION_CONNECTIONS_KEYS → automation.connections.keys).
type Config struct {
	Connections ConnectionsConfig `koanf:"connections"`
}

// ConfigPrefix returns the koanf prefix for automationconfig ("automation").
func (Config) ConfigPrefix() string { return "automation" }

// Validate checks that the keyring parses.
func (c *Config) Validate() error {
	if _, err := c.Keyring(); err != nil {
		return fmt.Errorf("automationconfig: connections.keys: %w", err)
	}
	return nil
}

// Keyring is the configured keyring; nil when no key is configured.
func (c *Config) Keyring() (connectors.Keyring, error) {
	return connectors.ParseKeyringActive(c.Connections.Keys, c.Connections.Active)
}
