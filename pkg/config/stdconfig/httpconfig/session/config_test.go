package session_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/session"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestDefaults_Session(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[session.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Duration != 720*time.Hour {
		t.Errorf("Duration default: got %v, want 720h", cfg.Duration)
	}
}

func TestValidate_NegativeDuration(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"http.session.duration": "-1h",
	}))
	_, err := config.Register[session.Config](r)
	if err == nil {
		t.Fatal("expected error for negative duration, got nil")
	}
}
