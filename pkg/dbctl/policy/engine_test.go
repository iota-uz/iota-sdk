package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEvaluate_DeniesHostNotAllowed(t *testing.T) {
	cfg := Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost", "127.0.0.1"},
				AllowDestructive: true,
			},
		},
		Credentials: CredentialPolicy{Emission: "token_only"},
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
		Credentials: CredentialPolicy{Emission: "token_only"},
	}

	decision := Evaluate(cfg, Target{Environment: "development", Host: "localhost"}, true)
	if decision.Allowed {
		t.Fatalf("expected policy denial for destructive operation")
	}
}

func TestEvaluate_RequiresYesAndTicket(t *testing.T) {
	cfg := Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost"},
				AllowDestructive: true,
				RequireYes:       true,
				RequireTicket:    true,
			},
		},
		Credentials: CredentialPolicy{Emission: "masked"},
	}

	decision := Evaluate(cfg, Target{Environment: "development", Host: "localhost"}, false)
	if !decision.Allowed {
		t.Fatalf("expected policy to allow operation")
	}
	if !decision.RequireYes {
		t.Fatalf("expected require yes to be true")
	}
	if !decision.RequireTicket {
		t.Fatalf("expected require ticket to be true")
	}
	if decision.CredentialEmission != "masked" {
		t.Fatalf("expected credential emission to be masked, got %q", decision.CredentialEmission)
	}
}

func TestLoad_RejectsUnsupportedCredentialEmission(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	payload := []byte(`
environments:
  development:
    allowed_hosts: ["localhost"]
    allow_destructive: true
credentials:
  emission: typo
`)
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write policy file: %v", err)
	}

	if _, _, err := Load(path); err == nil {
		t.Fatal("expected Load to reject unknown credentials.emission")
	}
}
