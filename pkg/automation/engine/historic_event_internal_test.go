package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// A stored event keeps only what today's schema declares: a field whose type changed (a scalar that was
// an object, a list that is now a string) does not carry its old nested data through.
func TestKeepDeclaredDropsDataOfAChangedType(t *testing.T) {
	t.Parallel()
	schema := expr.Fields{
		{Name: "note", Field: expr.StringField},
		{Name: "tags", Field: expr.ListOf(expr.StringField)},
		{Name: "client", Field: expr.Field{Kind: expr.Object, Props: []expr.Prop{{Name: "id", Field: expr.StringField}}}},
		{Name: "extra", Field: expr.OptionalOf(expr.Field{Kind: expr.Dyn})},
	}
	stored := map[string]any{
		"note":   map[string]any{"secret": "pin"}, // was an object
		"tags":   "a,b",                           // was a string
		"client": map[string]any{"id": "c-1", "ssn": "x"},
		"extra":  map[string]any{"any": "thing"},
		"gone":   "left the schema",
	}
	assert.Equal(t, map[string]any{
		"note":   nil,
		"tags":   nil,
		"client": map[string]any{"id": "c-1"},
		"extra":  map[string]any{"any": "thing"},
	}, keepDeclared(stored, schema))
}
