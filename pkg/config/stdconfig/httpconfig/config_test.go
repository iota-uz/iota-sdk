package httpconfig_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"http.port":                   8080,
		"http.domain":                 "example.com",
		"http.origin":                 "https://example.com",
		"http.allowedorigins":         []string{"https://app.example.com"},
		"http.headers.requestid":      "X-Req-ID",
		"http.headers.realip":         "X-Forwarded-For",
		"http.cookies.sid":            "mysid",
		"http.cookies.oauthstate":     "mystate",
		"http.session.duration":       "48h",
		"http.pagination.pagesize":    50,
		"http.pagination.maxpagesize": 200,
	})

	var cfg httpconfig.Config
	if err := src.Unmarshal("http", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Port != 8080 {
		t.Errorf("Port: want 8080, got %d", cfg.Port)
	}
	if cfg.Domain != "example.com" {
		t.Errorf("Domain: want example.com, got %s", cfg.Domain)
	}
	if cfg.OriginOverride != "https://example.com" {
		t.Errorf("OriginOverride: want https://example.com, got %s", cfg.OriginOverride)
	}
	if cfg.Headers.RequestID != "X-Req-ID" {
		t.Errorf("Headers.RequestID: want X-Req-ID, got %s", cfg.Headers.RequestID)
	}
	if cfg.Cookies.SID != "mysid" {
		t.Errorf("Cookies.SID: want mysid, got %s", cfg.Cookies.SID)
	}
	if cfg.Session.Duration != 48*time.Hour {
		t.Errorf("Session.Duration: want 48h, got %s", cfg.Session.Duration)
	}
	if cfg.Pagination.PageSize != 50 {
		t.Errorf("Pagination.PageSize: want 50, got %d", cfg.Pagination.PageSize)
	}
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[httpconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Port != 3200 {
		t.Errorf("Port default: want 3200, got %d", cfg.Port)
	}
	if cfg.Domain != "localhost" {
		t.Errorf("Domain default: want localhost, got %s", cfg.Domain)
	}
}

func TestConfig_Origin(t *testing.T) {
	t.Parallel()

	prodApp := &appconfig.Config{Environment: "production"}
	devApp := &appconfig.Config{Environment: "development"}

	// Override wins regardless of app env.
	cfg := &httpconfig.Config{OriginOverride: "https://pinned.example.com", Port: 443, Domain: "example.com"}
	if got := cfg.Origin(devApp); got != "https://pinned.example.com" {
		t.Errorf("OriginOverride: got %q", got)
	}

	// Production: no port.
	cfg2 := &httpconfig.Config{Port: 443, Domain: "example.com"}
	if got := cfg2.Origin(prodApp); got != "https://example.com" {
		t.Errorf("prod Origin: got %q, want https://example.com", got)
	}

	// Dev: with port.
	cfg3 := &httpconfig.Config{Port: 3200, Domain: "localhost"}
	if got := cfg3.Origin(devApp); got != "http://localhost:3200" {
		t.Errorf("dev Origin: got %q, want http://localhost:3200", got)
	}
}
