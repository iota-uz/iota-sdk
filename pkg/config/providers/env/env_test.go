package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
)

func writeEnvFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}
	return path
}

func TestEnv_DotEnvFileLoad(t *testing.T) {
	t.Parallel()

	path := writeEnvFile(t, "APP_HOST=localhost\nAPP_PORT=3000\n")
	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}

	if _, ok := src.Get("app.host"); !ok {
		t.Error("app.host should be present")
	}
	if _, ok := src.Get("app.port"); !ok {
		t.Error("app.port should be present")
	}
}

// TestEnv_ProcessEnvOverlayOverridesFile cannot use t.Parallel because
// t.Setenv and t.Parallel are mutually exclusive in Go 1.21+.
func TestEnv_ProcessEnvOverlayOverridesFile(t *testing.T) {
	path := writeEnvFile(t, "APP_HOST=from-file\n")

	// t.Setenv restores the original value when the test ends.
	t.Setenv("APP_HOST", "from-process")

	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var out struct {
		Host string `koanf:"host"`
	}
	if err := src.Unmarshal("app", &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Host != "from-process" {
		t.Errorf("process env should override file: got %q", out.Host)
	}
}

func TestEnv_KeyTransform_SingleUnderscoreToDot(t *testing.T) {
	t.Parallel()

	path := writeEnvFile(t, "BICHAT_OPENAI_API_KEY=sk-test\n")
	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, ok := src.Get("bichat.openai.api.key"); !ok {
		t.Error("BICHAT_OPENAI_API_KEY should map to bichat.openai.api.key")
	}
}

func TestEnv_KeyTransform_LeadingTrailingUnderscore(t *testing.T) {
	t.Parallel()

	path := writeEnvFile(t, "_LEADING=a\nTRAILING_=b\n")
	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if _, ok := src.Get("leading"); !ok {
		t.Error("_LEADING should map to leading")
	}
	if _, ok := src.Get("trailing"); !ok {
		t.Error("TRAILING_ should map to trailing")
	}
}

// TestEnv_EmptyValuesTreatedAsAbsent guards the strict-mode regression where
// the `${VAR:-}` container idiom exports optional settings as empty env vars.
// Such phantom keys must not land in the Source, or an unset optional feature
// looks "partially configured" (HasPrefix true, IsConfigured false) and trips
// a production strict-mode boot panic.
func TestEnv_EmptyValuesTreatedAsAbsent(t *testing.T) {
	t.Parallel()

	path := writeEnvFile(t, "TELEGRAM_CHAT_ID=\nTELEGRAM_CHAT_SOSTHREADID=\nAPP_HOST=localhost\n")
	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}

	if _, ok := src.Get("telegram.chat.id"); ok {
		t.Error("empty TELEGRAM_CHAT_ID must not be present in the source")
	}
	if src.HasPrefix("telegram.chat") {
		t.Error("a prefix whose only env vars are empty must not register as present")
	}
	// Non-empty vars are unaffected.
	if _, ok := src.Get("app.host"); !ok {
		t.Error("non-empty APP_HOST should still be present")
	}
}

// TestEnv_EmptyProcessEnvDoesNotOverrideFile documents that an empty process
// env var is treated as absent rather than overriding a non-empty file value.
func TestEnv_EmptyProcessEnvDoesNotOverrideFile(t *testing.T) {
	path := writeEnvFile(t, "APP_HOST=from-file\n")
	t.Setenv("APP_HOST", "")

	src, err := config.Build(New(path))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	v, ok := src.Get("app.host")
	if !ok || v != "from-file" {
		t.Errorf("empty process env should not erase file value: got %q ok=%v", v, ok)
	}
}

func TestEnv_MissingFileIsSilent(t *testing.T) {
	t.Parallel()

	src, err := config.Build(New("/does/not/exist/.env"))
	if err != nil {
		t.Fatalf("missing .env file should not error: %v", err)
	}
	_ = src
}

func TestEnv_MalformedFileReturnsError(t *testing.T) {
	t.Parallel()

	// An unterminated quoted value causes godotenv to return an error.
	path := writeEnvFile(t, "KEY='unclosed value\n")

	_, err := config.Build(New(path))
	if err == nil {
		t.Error("malformed .env file should return an error")
	}
}
