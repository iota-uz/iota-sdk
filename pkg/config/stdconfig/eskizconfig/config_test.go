package eskizconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/eskizconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"eskiz.url":      "https://notify.example.com",
		"eskiz.email":    "ops@example.com",
		"eskiz.password": "pw",
		"eskiz.from":     "4546",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var cfg eskizconfig.Config
	if err := src.Unmarshal("eskiz", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.URL != "https://notify.example.com" || cfg.Email != "ops@example.com" || cfg.Password != "pw" || cfg.From != "4546" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.IsConfigured() {
		t.Fatal("IsConfigured: want true")
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  eskizconfig.Config
		want bool
	}{
		{"both", eskizconfig.Config{Email: "a@b.c", Password: "p"}, true},
		{"no password", eskizconfig.Config{Email: "a@b.c"}, false},
		{"no email", eskizconfig.Config{Password: "p"}, false},
		{"empty", eskizconfig.Config{}, false},
	}
	for _, tc := range cases {
		if got := tc.cfg.IsConfigured(); got != tc.want {
			t.Errorf("%s: IsConfigured = %v, want %v", tc.name, got, tc.want)
		}
	}
	if (&eskizconfig.Config{}).DisabledReason() == "" {
		t.Error("DisabledReason is empty")
	}
}
