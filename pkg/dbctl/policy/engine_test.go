package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_UsesBuiltInDefaultsWhenPathEmpty(t *testing.T) {
	t.Parallel()

	cfg, payload, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") returned error: %v", err)
	}
	if len(payload) == 0 {
		t.Fatal("expected default policy payload to be non-empty")
	}
	dev, ok := cfg.Environments["development"]
	if !ok {
		t.Fatal("expected default development environment")
	}
	if !dev.AllowDestructive {
		t.Fatal("expected development to allow destructive operations")
	}
}

func TestEvaluate_DeniesHostNotAllowed(t *testing.T) {
	cfg := Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost", "127.0.0.1"},
				AllowDestructive: true,
			},
		},
	}

	decision := Evaluate(cfg, Target{Environment: "development", Host: "prod.db.local"}, false)
	if decision.Allowed {
		t.Fatalf("expected policy denial for disallowed host")
	}
}

func TestEvaluate_DeniesDestructiveWhenForbidden(t *testing.T) {
	cfg := Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost"},
				AllowDestructive: false,
			},
		},
	}

	decision := Evaluate(cfg, Target{Environment: "development", Host: "localhost"}, true)
	if decision.Allowed {
		t.Fatalf("expected policy denial for destructive operation")
	}
}

func TestEvaluate_AllowsNonDestructiveOperation(t *testing.T) {
	cfg := Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost"},
				AllowDestructive: true,
			},
		},
	}

	decision := Evaluate(cfg, Target{Environment: "development", Host: "localhost"}, false)
	if !decision.Allowed {
		t.Fatalf("expected policy to allow operation")
	}
}

func TestEvaluate_DeniesEmptyEnvironment(t *testing.T) {
	cfg := DefaultConfig()

	decision := Evaluate(cfg, Target{Environment: "", Host: "localhost"}, true)
	if decision.Allowed {
		t.Fatal("expected empty environment to be denied")
	}
}

func TestLoad_RejectsUnknownFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	payload := []byte("environments:\n  development:\n    allowed_hosts:\n      - localhost\n    unknown_flag: true\n")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, _, err := Load(path); err == nil {
		t.Fatal("expected strict YAML parsing to reject unknown fields")
	}
}
