package static

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

func TestStatic_FlatDottedKeys(t *testing.T) {
	t.Parallel()

	src, err := config.Build(New(map[string]any{
		"db.host": "localhost",
		"db.port": 5432,
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var out struct {
		Host string `koanf:"host"`
		Port int    `koanf:"port"`
	}
	if err := src.Unmarshal("db", &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Host != "localhost" {
		t.Errorf("Host: got %q, want %q", out.Host, "localhost")
	}
	if out.Port != 5432 {
		t.Errorf("Port: got %d, want %d", out.Port, 5432)
	}
}

func TestStatic_NestedMapValues(t *testing.T) {
	t.Parallel()

	src, err := config.Build(New(map[string]any{
		"app": map[string]any{
			"name": "myapp",
			"env":  "test",
		},
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var out struct {
		Name string `koanf:"name"`
		Env  string `koanf:"env"`
	}
	if err := src.Unmarshal("app", &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Name != "myapp" {
		t.Errorf("Name: got %q, want %q", out.Name, "myapp")
	}
	if out.Env != "test" {
		t.Errorf("Env: got %q, want %q", out.Env, "test")
	}
}

func TestStatic_EmptyInputIsNoop(t *testing.T) {
	t.Parallel()

	src, err := config.Build(New(nil))
	if err != nil {
		t.Fatalf("Build with nil map: %v", err)
	}
	if _, ok := src.Get("anything"); ok {
		t.Error("empty static provider should have no keys")
	}

	src2, err := config.Build(New(map[string]any{}))
	if err != nil {
		t.Fatalf("Build with empty map: %v", err)
	}
	if _, ok := src2.Get("anything"); ok {
		t.Error("empty static provider should have no keys")
	}
}
