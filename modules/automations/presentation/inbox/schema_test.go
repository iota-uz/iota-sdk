package inbox

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func TestFieldsOfKeepsOrderAndKinds(t *testing.T) {
	t.Parallel()
	fields := fieldsOf(json.RawMessage(`{"type":"object","required":["priority"],"properties":{
		"note":{"type":"string","maxLength":100},"amount":{"type":"number","title":"Amount"},"count":{"type":"integer"},
		"urgent":{"type":"boolean"},"priority":{"type":"string","enum":["low","high"]},"extra":{"type":"object"},"text":{"type":"string"}}}`))
	require.Len(t, fields, 7)
	names, kinds := make([]string, 0, len(fields)), make([]string, 0, len(fields))
	for _, f := range fields {
		names, kinds = append(names, f.Name), append(kinds, f.Kind)
	}
	assert.Equal(t, []string{"note", "amount", "count", "urgent", "priority", "extra", "text"}, names)
	assert.Equal(t, []string{"text", "number", "integer", "boolean", "enum", "json", "textarea"}, kinds)
	assert.Equal(t, "Amount", fields[1].Title)
	assert.True(t, fields[4].Required)
	assert.Nil(t, fieldsOf(nil))
}

func TestFieldsFromFormCoercesByKind(t *testing.T) {
	t.Parallel()
	fields := fieldsOf(json.RawMessage(`{"properties":{"amount":{"type":"number"},"count":{"type":"integer"},"urgent":{"type":"boolean"},
		"priority":{"enum":["low"]},"extra":{"type":"object"},"empty":{"type":"string","maxLength":10},"bad":{"type":"integer"}}}`))
	raw, kept := fieldsFromForm(fields, url.Values{
		"field.amount": {"12,5"}, "field.count": {"3"}, "field.urgent": {"true"}, "field.priority": {"low"},
		"field.extra": {`{"a":1}`}, "field.bad": {"x"},
	})
	assert.JSONEq(t, `{"amount":12.5,"count":3,"urgent":true,"priority":"low","extra":{"a":1},"bad":"x"}`, string(raw))
	require.Len(t, kept, 7)
	assert.Equal(t, "12,5", kept[0].Value, "the value entered is kept for a form shown again")
}

// A choice of an enum goes back with its type: an integer enum is chosen and passes its schema.
func TestEnumChoicesRoundTrip(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","required":["level","mode"],"properties":{
		"level":{"type":"integer","enum":[1,2]},"mode":{"type":"string","enum":["a","b"]}}}`)
	fields := fieldsOf(schema)
	require.Len(t, fields, 2)
	assert.Equal(t, []EnumOption{{Value: "1", Label: "1"}, {Value: "2", Label: "2"}}, fields[0].Enum)
	assert.Equal(t, []EnumOption{{Value: `"a"`, Label: "a"}, {Value: `"b"`, Label: "b"}}, fields[1].Enum)
	raw, _ := fieldsFromForm(fields, url.Values{"field.level": {fields[0].Enum[1].Value}, "field.mode": {fields[1].Enum[0].Value}})
	assert.JSONEq(t, `{"level":2,"mode":"a"}`, string(raw))
	require.NoError(t, graph.ValidateJSON(schema, raw))
}

// A declared null is a choice: sent as null, apart from leaving the field out.
func TestNullEnumChoice(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","required":["level"],"properties":{"level":{"enum":[null,1]},"only":{"enum":[null]}}}`)
	fields := fieldsOf(schema)
	require.Len(t, fields, 2)
	assert.Equal(t, []EnumOption{{Value: "null", Label: ""}, {Value: "1", Label: "1"}}, fields[0].Enum)
	assert.Equal(t, []EnumOption{{Value: "null", Label: ""}}, fields[1].Enum)
	raw, _ := fieldsFromForm(fields, url.Values{"field.level": {"null"}, "field.only": {"null"}})
	assert.JSONEq(t, `{"level":null,"only":null}`, string(raw))
	require.NoError(t, graph.ValidateJSON(schema, raw))
	raw, _ = fieldsFromForm(fields, url.Values{"field.level": {"1"}, "field.only": {""}})
	assert.JSONEq(t, `{"level":1}`, string(raw), "an empty choice leaves the field out")
	require.NoError(t, graph.ValidateJSON(schema, raw))
}

// A choice goes back exactly as the schema wrote it: a large integer is not rounded, a nested value
// keeps its shape.
func TestEnumChoiceIsExact(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","properties":{"big":{"enum":[9007199254740993, 1]},"nested":{"enum":[{"a":[1,2]},"x"]}}}`)
	fields := fieldsOf(schema)
	require.Len(t, fields, 2)
	raw, _ := fieldsFromForm(fields, url.Values{"field.big": {fields[0].Enum[0].Value}, "field.nested": {fields[1].Enum[0].Value}})
	assert.JSONEq(t, `{"big":9007199254740993,"nested":{"a":[1,2]}}`, string(raw))
	assert.Contains(t, string(raw), "9007199254740993")
	require.NoError(t, graph.ValidateJSON(schema, raw))
}
