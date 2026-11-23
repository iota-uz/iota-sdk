package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

func TestSensitivePathsWalkMapsAndLists(t *testing.T) {
	person := expr.ObjectOf(expr.Fields{{Name: "name", Field: expr.StringField}, {Name: "ssn", Field: expr.StringField}})
	public := expr.ObjectOf(expr.Fields{{Name: "name", Field: expr.StringField}})
	full := expr.Fields{{Name: "by_id", Field: expr.MapOf(person)}, {Name: "list", Field: expr.ListOf(person)}}
	seen := expr.Fields{{Name: "by_id", Field: expr.MapOf(public)}, {Name: "list", Field: expr.ListOf(public)}}
	paths := diffPaths(full, seen, "")
	assert.ElementsMatch(t, []string{"by_id.*.ssn", "list.ssn"}, paths)

	payload := map[string]any{
		"by_id": map[string]any{"a": map[string]any{"name": "A", "ssn": "1"}, "b": map[string]any{"name": "B", "ssn": "2"}},
		"list":  []any{map[string]any{"name": "C", "ssn": "3"}},
	}
	var found []any
	for _, p := range paths {
		valuesAt(payload, strings.Split(p, "."), func(v any) { found = append(found, v) })
	}
	assert.ElementsMatch(t, []any{"1", "2", "3"}, found, "references reach the values inside map entries")
	for _, p := range paths {
		removePath(payload, strings.Split(p, "."))
	}
	assert.Equal(t, map[string]any{"name": "A"}, payload["by_id"].(map[string]any)["a"])
	assert.Equal(t, map[string]any{"name": "C"}, payload["list"].([]any)[0])
}
