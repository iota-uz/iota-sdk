package httpconfig_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"http.port":                   8080,
		"http.domain":                 "example.com",
		"http.origin":                 "https://example.com",
		"http.allowedorigins":         []string{"https://app.example.com"},
		"http.environment":            "production",
		"http.headers.requestid":      "X-Req-ID",
		"http.headers.realip":         "X-Forwarded-For",
		"http.cookies.sid":            "mysid",
		"http.cookies.oauthstate":     "mystate",
		"http.session.duration":       "48h",
		"http.pagination.pagesize":    50,
		"http.pagination.maxpagesize": 200,
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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
	if cfg.Environment != "production" {
		t.Errorf("Environment: want production, got %s", cfg.Environment)
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

func TestConfig_Methods(t *testing.T) {
	t.Parallel()

	prod := &httpconfig.Config{Port: 443, Environment: "production"}
	if !prod.IsProduction() {
		t.Error("IsProduction() should return true")
	}
	if prod.IsDev() {
		t.Error("IsDev() should return false in production")
	}
	if prod.Scheme() != "https" {
		t.Errorf("Scheme(): want https, got %s", prod.Scheme())
	}
	if prod.SocketAddress() != ":443" {
		t.Errorf("SocketAddress(): want :443, got %s", prod.SocketAddress())
	}

	dev := &httpconfig.Config{Port: 3200, Environment: "development"}
	if dev.IsProduction() {
		t.Error("IsProduction() should return false")
	}
	if !dev.IsDev() {
		t.Error("IsDev() should return true")
	}
	if dev.Scheme() != "http" {
		t.Errorf("Scheme(): want http, got %s", dev.Scheme())
	}
	if dev.SocketAddress() != "localhost:3200" {
		t.Errorf("SocketAddress(): want localhost:3200, got %s", dev.SocketAddress())
	}
}
