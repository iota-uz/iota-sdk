package yamlfile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	return path
}

func TestYAML_HappyPath(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "db:\n  host: pg.local\n  port: 5432\n")
	src, err := config.Build(New(path))
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
	if out.Host != "pg.local" {
		t.Errorf("Host: got %q, want %q", out.Host, "pg.local")
	}
	if out.Port != 5432 {
		t.Errorf("Port: got %d, want %d", out.Port, 5432)
	}
}

func TestYAML_MissingFileIsNoop(t *testing.T) {
	t.Parallel()

	src, err := config.Build(New("/does/not/exist/config.yaml"))
	if err != nil {
		t.Fatalf("missing yaml file should not error: %v", err)
	}
	if src.Has("anything") {
		t.Error("missing file source should have no keys")
	}
}

func TestYAML_MalformedFileReturnsError(t *testing.T) {
	t.Parallel()

	path := writeYAML(t, "key: [\nbad yaml\n")
	_, err := config.Build(New(path))
	if err == nil {
		t.Error("malformed yaml should return an error")
	}
}
