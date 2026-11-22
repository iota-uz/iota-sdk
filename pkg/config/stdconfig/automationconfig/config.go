// Package automationconfig provides typed configuration for automation graphs: the keyring that
// encrypts connector credentials in automation.connections.
// Intended to be registered via config.Register[automationconfig.Config].
package automationconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
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

// PlatformConfig configures the platform credentials: keys of the application offered to tenants by
// the connector's platform policy, and OAuth2 client credentials.
type PlatformConfig struct {
	// Credentials is a JSON object of credentials by name (a connector kind or an OAuth2 client
	// credential name), each an object of string fields:
	//
	//	{"openai": {"api_key": "..."}, "google_oauth": {"client_id": "...", "client_secret": "..."}}
	Credentials string `koanf:"credentials" secret:"true"`
}

// Config holds automation settings.
// Env prefix: "automation" (e.g. AUTOMATION_CONNECTIONS_KEYS → automation.connections.keys).
type Config struct {
	Connections ConnectionsConfig `koanf:"connections"`
	Platform    PlatformConfig    `koanf:"platform"`
}

// ConfigPrefix returns the koanf prefix for automationconfig ("automation").
func (Config) ConfigPrefix() string { return "automation" }

// Validate checks that the keyring parses; no keys is valid (connections are then unavailable).
func (c *Config) Validate() error {
	if _, err := c.Keyring(); err != nil && !errors.Is(err, connectors.ErrNotConfigured) {
		return fmt.Errorf("automationconfig: connections.keys: %w", err)
	}
	if _, err := c.PlatformCredentials(); err != nil {
		return err
	}
	return nil
}

// PlatformCredentials parses platform.credentials; empty is none. Errors never quote values.
func (c *Config) PlatformCredentials() (map[catalog.ConnectorKind]connectors.PlatformCredential, error) {
	raw := strings.TrimSpace(c.Platform.Credentials)
	out := map[catalog.ConnectorKind]connectors.PlatformCredential{}
	if raw == "" {
		return out, nil
	}
	// Decoded as raw values first: a JSON null would otherwise pass as an empty object or string.
	shape := errors.New("automationconfig: platform.credentials is not a JSON object of objects of strings")
	var doc map[string]map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc == nil {
		return nil, shape
	}
	for name, fields := range doc {
		if fields == nil {
			return nil, shape
		}
		creds := connectors.Credentials{}
		for field, v := range fields {
			var s string
			if !bytes.HasPrefix(bytes.TrimSpace(v), []byte(`"`)) || json.Unmarshal(v, &s) != nil {
				return nil, shape
			}
			creds[field] = s
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, errors.New("automationconfig: platform.credentials has an entry without a name")
		}
		if _, dup := out[catalog.ConnectorKind(name)]; dup {
			// Two entries that are the same name once trimmed: which one wins would depend on map order.
			return nil, errors.New("automationconfig: platform.credentials names an entry twice")
		}
		out[catalog.ConnectorKind(name)] = connectors.PlatformCredential{Credentials: creds}
	}
	return out, nil
}

// Keyring is the configured keyring; connectors.ErrNotConfigured when no key is configured (pass a nil
// keyring to connectors.NewStore then).
func (c *Config) Keyring() (connectors.Keyring, error) {
	return connectors.ParseKeyringActive(c.Connections.Keys, c.Connections.Active)
}
