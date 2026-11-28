package automationconfig_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/automationconfig"
)

func TestKeyringFromConfig(t *testing.T) {
	t.Parallel()
	k2 := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("b", 32)))
	k1 := strings.Repeat("ab", 32)
	src, err := config.Build(static.New(map[string]any{"automation.connections.keys": "k2:" + k2 + ",k1:" + k1}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var cfg automationconfig.Config
	if err := src.Unmarshal("automation", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	kr, err := cfg.Keyring()
	if err != nil || kr == nil {
		t.Fatalf("Keyring: %v", err)
	}
	if id, _ := kr.Active(); id != "k2" {
		t.Fatalf("active key %s", id)
	}
	if _, ok := kr.Key("k1"); !ok {
		t.Fatal("k1 is not in the keyring")
	}
}

func TestEmptyKeysIsNoKeyring(t *testing.T) {
	t.Parallel()
	var cfg automationconfig.Config
	if kr, err := cfg.Keyring(); !errors.Is(err, connectors.ErrNotConfigured) || kr != nil {
		t.Fatalf("keyring %v, %v", kr, err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("no keys is valid: %v", err)
	}
	cfg.Connections.Keys = "k1:short"
	if err := cfg.Validate(); err == nil {
		t.Fatal("a short key is accepted")
	} else if strings.Contains(err.Error(), "short") {
		t.Fatalf("the error shows the key: %v", err)
	}
}

func TestPlatformCredentialsFromConfig(t *testing.T) {
	t.Parallel()
	cfg := automationconfig.Config{Platform: automationconfig.PlatformConfig{Credentials: `{"openai":{"api_key":"sk-secret"}}`}}
	creds, err := cfg.PlatformCredentials()
	if err != nil {
		t.Fatalf("PlatformCredentials: %v", err)
	}
	if creds["openai"].Credentials["api_key"] != "sk-secret" {
		t.Fatalf("credentials %v", creds)
	}
	cfg.Platform.Credentials = `{"openai":{"api_key":42}}`
	if err := cfg.Validate(); err == nil {
		t.Fatal("a non-string field is accepted")
	} else if strings.Contains(err.Error(), "42") {
		t.Fatalf("the error shows the value: %v", err)
	}
}

// JSON null anywhere and names that collide once trimmed are refused, without quoting names or values.
func TestPlatformCredentialsRefusesNullAndCollisions(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`null`, `{"openai":null}`, `{"openai":{"api_key":null}}`, `{"openai":{"api_key":"a"}," openai ":{"api_key":"b"}}`,
	} {
		cfg := automationconfig.Config{Platform: automationconfig.PlatformConfig{Credentials: raw}}
		_, err := cfg.PlatformCredentials()
		if err == nil {
			t.Fatalf("%s is accepted", raw)
		}
		if strings.Contains(err.Error(), "openai") || strings.Contains(err.Error(), "api_key") {
			t.Fatalf("the error names the entry: %v", err)
		}
	}
	cfg := automationconfig.Config{Platform: automationconfig.PlatformConfig{Credentials: `{"openai":{}}`}}
	if _, err := cfg.PlatformCredentials(); err != nil {
		t.Fatalf("an empty credential object: %v", err)
	}
}

func TestSecretsDefaultToTheSealedStore(t *testing.T) {
	t.Parallel()
	var cfg automationconfig.Config
	_, ok, err := cfg.SecretBackend()
	if err != nil || ok {
		t.Fatalf("backend %v, %v", ok, err)
	}
	loc, err := cfg.SecretLocator()
	if err != nil {
		t.Fatal(err)
	}
	if root := loc.Root(uuid.Nil); root.Path != "tenants/"+uuid.Nil.String()+"/automation/secrets/" {
		t.Fatalf("root %q", root.Path)
	}
}

func TestSecretsOpenBaoFromEnvironmentKeys(t *testing.T) {
	t.Parallel()
	src, err := config.Build(static.New(map[string]any{
		"automation.secrets.store":                       "openbao",
		"automation.secrets.prefix":                      "prod",
		"automation.secrets.openbao.address":             "https://openbao.internal:8200",
		"automation.secrets.openbao.writer.method":       "approle",
		"automation.secrets.openbao.writer.roleid":       "writer-role",
		"automation.secrets.openbao.writer.secretidfile": "/run/secrets/writer-secret-id",
		"automation.secrets.openbao.reader.method":       "kubernetes",
		"automation.secrets.openbao.reader.role":         "iota-reader",
	}))
	if err != nil {
		t.Fatal(err)
	}
	var cfg automationconfig.Config
	if err := src.Unmarshal("automation", &cfg); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	backend, ok, err := cfg.SecretBackend()
	if err != nil || !ok || backend.Writer == nil || backend.Reader == nil || backend.Metadata == nil {
		t.Fatalf("backend %+v, %v", backend, err)
	}
	// The parts are separate: the writer cannot be used to read.
	if _, ok := backend.Writer.(secrets.Reader); ok {
		t.Fatal("the writer is a reader")
	}
	loc, _ := cfg.SecretLocator()
	tenant := uuid.New()
	if root := loc.Root(tenant); root.Path != "prod/tenants/"+tenant.String()+"/automation/secrets/" {
		t.Fatalf("root %q", root.Path)
	}
}

func TestSecretsConfigErrorsNameNoCredential(t *testing.T) {
	t.Parallel()
	for name, s := range map[string]automationconfig.SecretsConfig{
		"unknown store": {Store: "env"},
		"no address": {Store: "openbao", OpenBao: automationconfig.OpenBaoConfig{
			Writer: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-writer-token"},
			Reader: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-reader-token"}}},
		"no reader": {Store: "openbao", OpenBao: automationconfig.OpenBaoConfig{Address: "http://bao:8200",
			Writer: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-writer-token"}}},
		"token twice": {Store: "openbao", OpenBao: automationconfig.OpenBaoConfig{Address: "http://bao:8200",
			Writer: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-writer-token", TokenFile: "/x"},
			Reader: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-reader-token"}}},
		"approle without role": {Store: "openbao", OpenBao: automationconfig.OpenBaoConfig{Address: "http://bao:8200",
			Writer: automationconfig.OpenBaoAuthConfig{Method: "approle", SecretID: "the-secret-id"},
			Reader: automationconfig.OpenBaoAuthConfig{Method: "token", Token: "hvs.the-reader-token"}}},
		"bad prefix": {Prefix: "prod/../other"},
	} {
		cfg := automationconfig.Config{Secrets: s}
		err := cfg.Validate()
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		for _, v := range []string{"the-writer-token", "the-reader-token", "the-secret-id"} {
			if strings.Contains(err.Error(), v) {
				t.Errorf("%s: the error quotes a credential: %v", name, err)
			}
		}
	}
}
