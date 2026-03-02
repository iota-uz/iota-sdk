package policy

import "testing"

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
