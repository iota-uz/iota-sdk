package graph_test

import (
	"encoding/json"
	"regexp"
	"testing"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func namePattern(t *testing.T) string {
	t.Helper()
	var s struct {
		Properties struct {
			Name struct {
				Pattern string `json:"pattern"`
			} `json:"name"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(graph.JSONSchema(), &s); err != nil {
		t.Fatal(err)
	}
	return s.Properties.Name.Pattern
}

// The schema's whitespace is exactly what TrimName trims, for every code point.
func TestNamePatternWhitespaceParity(t *testing.T) {
	re := regexp.MustCompile(namePattern(t))
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		name := string(r)
		if got, want := re.MatchString(name), !unicode.IsSpace(r); got != want {
			t.Fatalf("U+%04X: pattern %v, TrimName says a name %v", r, got, want)
		}
		if got := re.MatchString("a" + name + "a"); !got {
			t.Fatalf("U+%04X inside a name is refused", r)
		}
	}
}
