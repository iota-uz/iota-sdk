package uploadsconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/uploadsconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

func TestSetDefaults_ZeroValues(t *testing.T) {
	t.Parallel()

	cfg := uploadsconfig.Config{}
	cfg.SetDefaults()

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

func TestSetDefaults_NonZeroValuesUnchanged(t *testing.T) {
	t.Parallel()

	cfg := uploadsconfig.Config{
		Path:      "/custom",
		MaxSize:   1024,
		MaxMemory: 2048,
	}
	cfg.SetDefaults()

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

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		UploadsPath:     "/uploads",
		MaxUploadSize:   int64(67108864),
		MaxUploadMemory: int64(16777216),
	}

	got := uploadsconfig.FromLegacy(legacy)

	if got.Path != "/uploads" {
		t.Errorf("Path: got %q, want %q", got.Path, "/uploads")
	}
	if got.MaxSize != 67108864 {
		t.Errorf("MaxSize: got %d, want 67108864", got.MaxSize)
	}
	if got.MaxMemory != 16777216 {
		t.Errorf("MaxMemory: got %d, want 16777216", got.MaxMemory)
	}
}

func TestFromLegacy_Defaults(t *testing.T) {
	t.Parallel()

	// Empty legacy fields → defaults applied.
	legacy := &configuration.Configuration{}
	got := uploadsconfig.FromLegacy(legacy)

	if got.Path != "static" {
		t.Errorf("Path default via FromLegacy: got %q, want %q", got.Path, "static")
	}
	if got.MaxSize != 33554432 {
		t.Errorf("MaxSize default via FromLegacy: got %d, want 33554432", got.MaxSize)
	}
	if got.MaxMemory != 33554432 {
		t.Errorf("MaxMemory default via FromLegacy: got %d, want 33554432", got.MaxMemory)
	}
}
