package uploadsconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"uploads.path":      "/var/uploads",
		"uploads.maxsize":   int64(10485760),
		"uploads.maxmemory": int64(5242880),
	})

	var cfg uploadsconfig.Config
	if err := src.Unmarshal("uploads", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Path != "/var/uploads" {
		t.Errorf("Path: got %q, want %q", cfg.Path, "/var/uploads")
	}
	if cfg.MaxSize != 10485760 {
		t.Errorf("MaxSize: got %d, want 10485760", cfg.MaxSize)
	}
	if cfg.MaxMemory != 5242880 {
		t.Errorf("MaxMemory: got %d, want 5242880", cfg.MaxMemory)
	}
}

func TestDefaults_ZeroValues(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[uploadsconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Path != "static" {
		t.Errorf("Path default: got %q, want %q", cfg.Path, "static")
	}
	if cfg.MaxSize != 33554432 {
		t.Errorf("MaxSize default: got %d, want 33554432", cfg.MaxSize)
	}
	if cfg.MaxMemory != 33554432 {
		t.Errorf("MaxMemory default: got %d, want 33554432", cfg.MaxMemory)
	}
}

func TestDefaults_NonZeroValuesUnchanged(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"uploads.path":      "/custom",
		"uploads.maxsize":   int64(1024),
		"uploads.maxmemory": int64(2048),
	}))
	cfg, err := config.Register[uploadsconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.Path != "/custom" {
		t.Errorf("Path should be unchanged: got %q", cfg.Path)
	}
	if cfg.MaxSize != 1024 {
		t.Errorf("MaxSize should be unchanged: got %d", cfg.MaxSize)
	}
	if cfg.MaxMemory != 2048 {
		t.Errorf("MaxMemory should be unchanged: got %d", cfg.MaxMemory)
	}
}
