package stdconfig_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig"
)

// updateGolden regenerates the golden fixture when run with -update.
// Usage: go test -run GoldenDefaults -update ./pkg/config/stdconfig/
var updateGolden = flag.Bool("update", false, "regenerate golden fixture files")

// TestGoldenDefaults verifies that the default values produced by all
// stdconfig packages remain stable. If the fixture does not exist or
// -update is passed, the test generates it and passes; otherwise it
// compares the current output to the committed fixture.
//
// Any unintentional change to a default value (including newly added
// packages that lack defaults) will be caught here.
func TestGoldenDefaults(t *testing.T) {
	t.Parallel()

	// Build an empty source so all fields get their tag-based defaults.
	src, err := config.Build(static.New(nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	registry := config.NewRegistry(src)
	bundle, err := stdconfig.RegisterAll(registry)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	got, err := json.MarshalIndent(bundle, "", "  ") //nolint:musttag // stdconfig types use koanf tags, not json — fixture compares Go field names
	if err != nil {
		t.Fatalf("json.MarshalIndent: %v", err)
	}
	got = append(got, '\n') // trailing newline for diff-friendliness

	fixturePath := filepath.Join("testdata", "defaults.golden.json")

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(fixturePath, got, 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		t.Logf("golden fixture updated: %s", fixturePath)
		return
	}

	want, err := os.ReadFile(fixturePath)
	if os.IsNotExist(err) {
		t.Fatalf("golden fixture missing; run with -update to generate: %s", fixturePath)
	}
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("golden defaults mismatch — run with -update to regenerate\ndiff (got vs want):\n%s", diff(string(want), string(got)))
	}
}

// diff produces a simple line-diff for readability in test output.
func diff(want, got string) string {
	wantLines := splitLines(want)
	gotLines := splitLines(got)

	var out []string
	maxLen := len(wantLines)
	if len(gotLines) > maxLen {
		maxLen = len(gotLines)
	}

	for i := range maxLen {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			out = append(out, "- "+w)
			out = append(out, "+ "+g)
		}
	}
	if len(out) == 0 {
		return "(no diff)"
	}
	result := ""
	for _, l := range out {
		result += l + "\n"
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, c := range s {
		if c == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
