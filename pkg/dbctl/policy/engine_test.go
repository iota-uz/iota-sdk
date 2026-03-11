package policy

import "testing"

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
	if !dev.RequireYes {
		t.Fatal("expected development to require yes")
	}
	if dev.RequireTicket {
		t.Fatal("expected development not to require ticket")
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
}
