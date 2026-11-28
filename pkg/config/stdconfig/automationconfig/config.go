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
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets/openbao"
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

// SecretsConfig configures where the values of the tenants' secrets (the «Secret» connector) live.
type SecretsConfig struct {
	// Store is "sealed" (default: the connections' keyring, in PostgreSQL) or "openbao".
	Store string `koanf:"store"`
	// Prefix is the environment the paths of the values start with ("prod"): the locator forms
	// "<prefix>/tenants/<tenant>/automation/secrets/<secret>", and an OpenBao policy is written against it.
	Prefix  string        `koanf:"prefix"`
	OpenBao OpenBaoConfig `koanf:"openbao"`
}

// OpenBaoConfig is the OpenBao (or Vault) KV v2 server of the secrets' values and the two service
// identities the application uses there: Writer manages values (the integrations page) and may not read
// them; Reader reads them (the nodes). Each logs in on its own and renews its token while running.
type OpenBaoConfig struct {
	// Address is the server's URL ("https://openbao.internal:8200").
	Address string `koanf:"address"`
	// Mount is the KV v2 mount (default "secret").
	Mount     string            `koanf:"mount"`
	Namespace string            `koanf:"namespace"`
	Writer    OpenBaoAuthConfig `koanf:"writer"`
	Reader    OpenBaoAuthConfig `koanf:"reader"`
}

// OpenBaoAuthConfig is how one service identity logs in. A credential given as a file is read on every
// login, so a rotated secret_id, JWT or agent-written token applies without a restart.
type OpenBaoAuthConfig struct {
	// Method is "approle", "kubernetes", "jwt" or "token".
	Method string `koanf:"method"`
	// AuthMount is the auth method's mount (default: the method's name).
	AuthMount string `koanf:"authmount"`
	// RoleID and SecretID (or SecretIDFile) log in with AppRole.
	RoleID       string `koanf:"roleid"`
	SecretID     string `koanf:"secretid" secret:"true"`
	SecretIDFile string `koanf:"secretidfile"`
	// Role and JWTFile log in with Kubernetes (JWTFile default: the pod's service-account token) or JWT.
	Role    string `koanf:"role"`
	JWTFile string `koanf:"jwtfile"`
	// Token (or TokenFile, an agent's sink) is a token given to the service.
	Token     string `koanf:"token" secret:"true"`
	TokenFile string `koanf:"tokenfile"`
}

// Config holds automation settings.
// Env prefix: "automation" (e.g. AUTOMATION_CONNECTIONS_KEYS → automation.connections.keys).
type Config struct {
	Connections ConnectionsConfig `koanf:"connections"`
	Platform    PlatformConfig    `koanf:"platform"`
	Secrets     SecretsConfig     `koanf:"secrets"`
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
	if _, err := c.SecretLocator(); err != nil {
		return err
	}
	if _, _, err := c.SecretBackend(); err != nil {
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

// Secret stores.
const (
	SecretStoreSealed  = "sealed"
	SecretStoreOpenBao = "openbao"
)

// kubernetesTokenFile is where a pod's service-account token is projected.
const kubernetesTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// SecretLocator is the locator of the secrets' values: secrets.DefaultLocator under secrets.prefix.
func (c *Config) SecretLocator() (secrets.Locator, error) {
	prefix := strings.Trim(strings.TrimSpace(c.Secrets.Prefix), "/")
	if prefix == "" {
		return secrets.DefaultLocator(""), nil
	}
	for _, seg := range strings.Split(prefix, "/") {
		if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, "?#%\\ ") {
			return nil, errors.New("automationconfig: secrets.prefix is not a clean path")
		}
	}
	return secrets.DefaultLocator(prefix + "/"), nil
}

// SecretBackend is the value store of the tenants' secrets by part, when secrets.store is "openbao":
// the writer and the metadata over the writer identity, the reader over the reader identity.
type SecretBackend struct {
	Writer   secrets.Writer
	Reader   secrets.Reader
	Metadata secrets.MetadataReader
}

// SecretBackend builds the configured OpenBao backend; ok is false for the sealed store. It contacts
// nothing.
func (c *Config) SecretBackend() (SecretBackend, bool, error) {
	switch strings.ToLower(strings.TrimSpace(c.Secrets.Store)) {
	case "", SecretStoreSealed:
		return SecretBackend{}, false, nil
	case SecretStoreOpenBao:
	default:
		return SecretBackend{}, false, fmt.Errorf("automationconfig: secrets.store is %q, not %q or %q", c.Secrets.Store, SecretStoreSealed, SecretStoreOpenBao)
	}
	ob := c.Secrets.OpenBao
	client := func(role string, a OpenBaoAuthConfig) (*openbao.Client, error) {
		auth, err := a.auth()
		if err != nil {
			return nil, fmt.Errorf("automationconfig: secrets.openbao.%s: %w", role, err)
		}
		cl, err := openbao.New(openbao.Config{Address: ob.Address, Mount: ob.Mount, Namespace: ob.Namespace, Auth: auth})
		if err != nil {
			return nil, fmt.Errorf("automationconfig: secrets.openbao: %w", err)
		}
		return cl, nil
	}
	w, err := client("writer", ob.Writer)
	if err != nil {
		return SecretBackend{}, false, err
	}
	r, err := client("reader", ob.Reader)
	if err != nil {
		return SecretBackend{}, false, err
	}
	return SecretBackend{Writer: w.Writer(), Reader: r.Reader(), Metadata: w.MetadataReader()}, true, nil
}

// credential is a credential given as a value or as a file; ok is false when it is given as neither.
func credential(literal, file, what string) (openbao.Credential, bool, error) {
	literal, file = strings.TrimSpace(literal), strings.TrimSpace(file)
	switch {
	case literal != "" && file != "":
		return nil, false, fmt.Errorf("%s is given both as a value and as a file", what)
	case file != "":
		return openbao.File(file), true, nil
	case literal != "":
		return openbao.Literal(literal), true, nil
	}
	return nil, false, nil
}

func (a OpenBaoAuthConfig) auth() (openbao.Auth, error) {
	switch strings.ToLower(strings.TrimSpace(a.Method)) {
	case "approle":
		if strings.TrimSpace(a.RoleID) == "" {
			return nil, errors.New("roleid is required for approle")
		}
		// No secret_id is a role that binds none (bind_secret_id = false).
		secret, _, err := credential(a.SecretID, a.SecretIDFile, "secretid")
		if err != nil {
			return nil, err
		}
		return openbao.AppRoleAuth(a.AuthMount, openbao.Literal(strings.TrimSpace(a.RoleID)), secret), nil
	case "kubernetes", "jwt":
		if strings.TrimSpace(a.Role) == "" {
			return nil, fmt.Errorf("role is required for %s", a.Method)
		}
		file := strings.TrimSpace(a.JWTFile)
		if file == "" {
			if strings.EqualFold(a.Method, "jwt") {
				return nil, errors.New("jwtfile is required for jwt")
			}
			file = kubernetesTokenFile
		}
		if strings.EqualFold(a.Method, "jwt") {
			return openbao.JWTAuth(a.AuthMount, a.Role, openbao.File(file)), nil
		}
		return openbao.KubernetesAuth(a.AuthMount, a.Role, openbao.File(file)), nil
	case "token":
		tok, ok, err := credential(a.Token, a.TokenFile, "token")
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errors.New("token or tokenfile is required for token")
		}
		return openbao.TokenAuth(tok), nil
	case "":
		return nil, errors.New("method is required (approle, kubernetes, jwt or token)")
	default:
		return nil, fmt.Errorf("method %q is not approle, kubernetes, jwt or token", a.Method)
	}
}
