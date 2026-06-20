package headers_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestDefaults_Headers(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[headers.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.RequestID != "X-Request-ID" {
		t.Errorf("RequestID default: got %q, want \"X-Request-ID\"", cfg.RequestID)
	}
	if cfg.RealIP != "X-Real-IP" {
		t.Errorf("RealIP default: got %q, want \"X-Real-IP\"", cfg.RealIP)
	}
}

func TestRoundTrip_Headers(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"http.headers.requestid": "X-Custom-Request-ID",
		"http.headers.realip":    "X-Forwarded-For",
	}))
	cfg, err := config.Register[headers.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.RequestID != "X-Custom-Request-ID" {
		t.Errorf("RequestID: got %q", cfg.RequestID)
	}
	if cfg.RealIP != "X-Forwarded-For" {
		t.Errorf("RealIP: got %q", cfg.RealIP)
	}
}
