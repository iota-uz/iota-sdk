package details

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayoutDetailsPreservesValueOwnership(t *testing.T) {
	t.Parallel()

	input := map[string]any{
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "original"}},
	}
	entity := NewPayoutDetails(PayoutWithData(input))

	input["nested"].(map[string]any)["value"] = "mutated"
	input["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	assertNestedPayoutValues(t, entity, "original")

	exposed := entity.Data()
	exposed["nested"].(map[string]any)["value"] = "mutated"
	exposed["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	assertNestedPayoutValues(t, entity, "original")

	nested := entity.Get("nested").(map[string]any)
	nested["value"] = "mutated"
	assertNestedPayoutValues(t, entity, "original")
}

func TestPayoutDetailsSettersPreserveValueOwnership(t *testing.T) {
	t.Parallel()

	original := NewPayoutDetails(PayoutWithData(map[string]any{
		"nested": map[string]any{"value": "original"},
		"items":  []any{map[string]any{"value": "original"}},
	}))
	replacement := map[string]any{
		"nested": map[string]any{"value": "replacement"},
		"items":  []any{map[string]any{"value": "replacement"}},
	}

	replaced := original.SetData(replacement)
	replacement["nested"].(map[string]any)["value"] = "mutated"
	replacement["items"].([]any)[0].(map[string]any)["value"] = "mutated"
	assertNestedPayoutValues(t, replaced, "replacement")
	assertNestedPayoutValues(t, original, "original")

	value := map[string]any{"value": "added"}
	updated := original.Set("added", value)
	value["value"] = "mutated"
	require.IsType(t, map[string]any{}, updated.Get("added"))
	assert.Equal(t, "added", updated.Get("added").(map[string]any)["value"])
	assert.Nil(t, original.Get("added"))
}

func TestPayoutDetailsSetAllocatesAfterNilData(t *testing.T) {
	t.Parallel()

	entity := NewPayoutDetails(PayoutWithData(nil)).Set("key", "value")
	assert.Equal(t, "value", entity.Get("key"))
}

func TestPayoutDetailsClonesCyclicContainers(t *testing.T) {
	t.Parallel()

	cyclicMap := make(map[string]any)
	cyclicMap["self"] = cyclicMap
	cyclicSlice := make([]any, 1)
	cyclicSlice[0] = cyclicSlice

	entity := NewPayoutDetails(PayoutWithData(map[string]any{
		"map":   cyclicMap,
		"slice": cyclicSlice,
	}))
	data := entity.Data()

	clonedMap := data["map"].(map[string]any)
	assert.Equal(t, reflect.ValueOf(clonedMap).Pointer(), reflect.ValueOf(clonedMap["self"]).Pointer())
	clonedSlice := data["slice"].([]any)
	assert.Equal(t, reflect.ValueOf(clonedSlice).Pointer(), reflect.ValueOf(clonedSlice[0]).Pointer())
}

func TestPayoutDetailsClonesJSONStructFields(t *testing.T) {
	t.Parallel()

	type payload struct {
		Values map[string]string `json:"values"`
		Items  []string          `json:"items"`
	}
	input := payload{
		Values: map[string]string{"key": "original"},
		Items:  []string{"original"},
	}
	entity := NewPayoutDetails(PayoutWithData(map[string]any{"payload": input}))

	input.Values["key"] = "mutated"
	input.Items[0] = "mutated"
	cloned := entity.Get("payload").(payload)
	assert.Equal(t, "original", cloned.Values["key"])
	assert.Equal(t, "original", cloned.Items[0])

	cloned.Values["key"] = "mutated"
	cloned.Items[0] = "mutated"
	cloned = entity.Get("payload").(payload)
	assert.Equal(t, "original", cloned.Values["key"])
	assert.Equal(t, "original", cloned.Items[0])
}

func assertNestedPayoutValues(t *testing.T, entity PayoutDetails, expected string) {
	t.Helper()
	data := entity.Data()
	require.IsType(t, map[string]any{}, data["nested"])
	require.IsType(t, []any{}, data["items"])
	assert.Equal(t, expected, data["nested"].(map[string]any)["value"])
	assert.Equal(t, expected, data["items"].([]any)[0].(map[string]any)["value"])
}
