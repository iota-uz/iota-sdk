package automationconfig_test

import (
	"encoding/base64"
	"strings"
	"testing"

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
	kr, err := cfg.Keyring()
	if err != nil || kr != nil {
		t.Fatalf("keyring %v, %v", kr, err)
	}
	cfg.Connections.Keys = "k1:short"
	if err := cfg.Validate(); err == nil {
		t.Fatal("a short key is accepted")
	} else if strings.Contains(err.Error(), "short") {
		t.Fatalf("the error shows the key: %v", err)
	}
}
