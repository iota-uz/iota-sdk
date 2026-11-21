package automationconfig_test

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
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
