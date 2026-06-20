package ratelimitconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/ratelimitconfig"
)

// TestSetDefaults_PartialOverride verifies that setting only one field via
// a provider does not prevent other fields from receiving their defaults.
// Regression: the old all-zero gate would skip Storage="memory" when GlobalRPS
// was already populated by the source.
func TestSetDefaults_PartialOverride(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"ratelimit.globalrps": 500,
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	r := config.NewRegistry(src)
	cfg, err := config.Register[ratelimitconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.GlobalRPS != 500 {
		t.Errorf("GlobalRPS: got %d, want 500 (provider value)", cfg.GlobalRPS)
	}
	if cfg.Storage != "memory" {
		t.Errorf("Storage: got %q, want %q (default)", cfg.Storage, "memory")
	}
}
