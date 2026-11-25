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
	raw, kept, err := fieldsFromForm(fields, url.Values{
		"field.amount": {"12,5"}, "field.count": {"3"}, "field.urgent": {"true"}, "field.priority": {"low"},
		"field.extra": {`{"a":1}`}, "field.bad": {"x"},
	})
	require.NoError(t, err)
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
	raw, _, _ := fieldsFromForm(fields, url.Values{"field.level": {fields[0].Enum[1].Value}, "field.mode": {fields[1].Enum[0].Value}})
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
	raw, _, _ := fieldsFromForm(fields, url.Values{"field.level": {"null"}, "field.only": {"null"}})
	assert.JSONEq(t, `{"level":null,"only":null}`, string(raw))
	require.NoError(t, graph.ValidateJSON(schema, raw))
	raw, _, _ = fieldsFromForm(fields, url.Values{"field.level": {"1"}, "field.only": {""}})
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
	raw, _, _ := fieldsFromForm(fields, url.Values{"field.big": {fields[0].Enum[0].Value}, "field.nested": {fields[1].Enum[0].Value}})
	assert.JSONEq(t, `{"big":9007199254740993,"nested":{"a":[1,2]}}`, string(raw))
	assert.Contains(t, string(raw), "9007199254740993")
	require.NoError(t, graph.ValidateJSON(schema, raw))
}

// An untitled property is labelled by its name with its first character, not its first byte,
// uppercased.
func TestHumanizeUppercasesTheFirstRune(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Сумма платежа", humanize("сумма_платежа"))
	assert.Equal(t, "Due date", humanize("due-date"))
	assert.Equal(t, "Ünit", humanize("ünit"))
	assert.Empty(t, humanize(""))
}

// A number goes back as typed: no float rounding, an integer beyond int64, a decimal comma.
func TestNumbersKeepTheirValue(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","properties":{"big":{"type":"number"},"huge":{"type":"integer"},
		"comma":{"type":"number"},"plus":{"type":"number"},"lead":{"type":"integer"},"exp":{"type":"number"},"dot":{"type":"number"}}}`)
	fields := fieldsOf(schema)
	raw, _, err := fieldsFromForm(fields, url.Values{
		"field.big": {"9007199254740993"}, "field.huge": {"123456789012345678901234567890"}, "field.comma": {"12,5"},
		"field.plus": {"+0.10"}, "field.lead": {"007"}, "field.exp": {"1.5E3"}, "field.dot": {".5"},
	})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"big":9007199254740993`)
	assert.Contains(t, string(raw), `"huge":123456789012345678901234567890`)
	assert.Contains(t, string(raw), `"comma":12.5`)
	assert.Contains(t, string(raw), `"plus":0.10`)
	assert.Contains(t, string(raw), `"lead":7`)
	assert.Contains(t, string(raw), `"exp":1.5E3`)
	assert.Contains(t, string(raw), `"dot":0.5`)
	require.NoError(t, graph.ValidateJSON(schema, raw))

	// Text that is no number stays text, and the schema refuses it.
	raw, _, err = fieldsFromForm(fields, url.Values{"field.huge": {"12,5"}, "field.big": {"1.2.3"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"huge":"12,5","big":"1.2.3"}`, string(raw))
	require.Error(t, graph.ValidateJSON(schema, raw))
}

// NaN and Infinity are no JSON numbers: the submission is refused, never sent without its fields.
func TestNonFiniteNumbersAreRefused(t *testing.T) {
	t.Parallel()
	fields := fieldsOf(json.RawMessage(`{"type":"object","properties":{"amount":{"type":"number"},"note":{"type":"string","maxLength":10}}}`))
	for _, v := range []string{"NaN", "Inf", "-Infinity", "+inf"} {
		raw, kept, err := fieldsFromForm(fields, url.Values{"field.amount": {v}, "field.note": {"kept"}})
		require.Error(t, err, v)
		assert.Nil(t, raw, v)
		require.Len(t, kept, 2)
		assert.Equal(t, v, kept[0].Value, "what was typed is shown again")
		assert.Equal(t, "kept", kept[1].Value)
	}
}

// An optional boolean is unset, yes or no; unset leaves it out. A required one is a checkbox whose
// unchecked state is an explicit false.
func TestOptionalBooleanCanStayUnset(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","required":["agree"],"properties":{"urgent":{"type":"boolean"},"agree":{"type":"boolean"},"reason":{"type":"string","maxLength":50}},
		"dependentRequired":{"urgent":["reason"]}}`)
	fields := fieldsOf(schema)
	require.Len(t, fields, 3)
	raw, _, err := fieldsFromForm(fields, url.Values{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"agree":false}`, string(raw), "the optional boolean is left out, the required one is false")
	require.NoError(t, graph.ValidateJSON(schema, raw))

	raw, _, err = fieldsFromForm(fields, url.Values{"field.urgent": {"false"}, "field.agree": {"true"}, "field.reason": {"r"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"urgent":false,"agree":true,"reason":"r"}`, string(raw))
	raw, _, err = fieldsFromForm(fields, url.Values{"field.urgent": {"true"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"urgent":true,"agree":false}`, string(raw))
	raw, _, err = fieldsFromForm(fields, url.Values{"field.urgent": {"maybe"}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"urgent":"maybe","agree":false}`, string(raw), "anything else is refused by the schema")
}

// A schema whose fields are not its own properties gets one JSON control for the whole fields object,
// checked by the same schema; a schema that asks for nothing gets none.
func TestSchemaWithoutPropertiesGetsADocumentControl(t *testing.T) {
	t.Parallel()
	for _, schema := range []string{
		`{"type":"object","$ref":"#/$defs/r","$defs":{"r":{"required":["reason"],"properties":{"reason":{"type":"string"}}}}}`,
		`{"type":"object","allOf":[{"required":["reason"],"properties":{"reason":{"type":"string"}}}]}`,
		`{"type":"object","required":["reason"]}`,
		`{"type":"object","properties":{},"allOf":[{"required":["reason"]}]}`,
		`{"type":"object","properties":{"note":{"type":"string"}},"allOf":[{"required":["reason"]}]}`,
		`{"type":"object","properties":{"note":{"type":"string"}},"required":["reason"]}`,
	} {
		fields := fieldsOf(json.RawMessage(schema))
		require.Len(t, fields, 1, schema)
		assert.Equal(t, KindDocument, fields[0].Kind, schema)

		typed := ` {"reason":"late"} ` // as typed, spaces included
		raw, kept, err := fieldsFromForm(fields, url.Values{"fields": {typed}})
		require.NoError(t, err, schema)
		assert.JSONEq(t, `{"reason":"late"}`, string(raw))
		require.NoError(t, graph.ValidateJSON(json.RawMessage(schema), raw), schema)
		assert.Equal(t, typed, kept[0].Value, "the control keeps the text as typed")

		_, kept, err = fieldsFromForm(fields, url.Values{"fields": {`{"reason":`}})
		require.Error(t, err, "a document that is not JSON is refused")
		assert.Equal(t, `{"reason":`, kept[0].Value)

		raw, _, err = fieldsFromForm(fields, url.Values{})
		require.NoError(t, err)
		assert.Nil(t, raw, "nothing typed sends no fields, and the schema decides")
	}
	for _, schema := range []string{`{"type":"object"}`, `{"type":"object","properties":{},"additionalProperties":false}`, `{"title":"x"}`} {
		assert.Empty(t, fieldsOf(json.RawMessage(schema)), schema)
	}
	assert.Len(t, fieldsOf(json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"}},"allOf":[{}]}`)), 1,
		"a schema with properties keeps its controls")
}

// A mixed schema of a named property and dynamic keys: the property controls alone cannot meet it (the
// only submission they make is refused), the whole-document control can.
func TestMixedNamedAndDynamicKeysGetADocumentControl(t *testing.T) {
	t.Parallel()
	schema := json.RawMessage(`{"type":"object","properties":{"note":{"type":"string"}},"patternProperties":{"^detail_":{"type":"string"}},"additionalProperties":false,"minProperties":2}`)
	require.Error(t, graph.ValidateJSON(schema, json.RawMessage(`{"note":"ok"}`)), "what the note control alone submits")
	fields := fieldsOf(schema)
	require.Len(t, fields, 1)
	assert.Equal(t, KindDocument, fields[0].Kind)
	raw, _, err := fieldsFromForm(fields, url.Values{"fields": {`{"note":"ok","detail_a":"ok"}`}})
	require.NoError(t, err)
	require.NoError(t, graph.ValidateJSON(schema, raw))

	// Named properties only, with additionalProperties and dependentRequired, keep their controls.
	plain := json.RawMessage(`{"type":"object","properties":{"a":{"type":"string"},"b":{"type":"string"}},"additionalProperties":false,"dependentRequired":{"a":["b"]},"x-iota-hint":"h"}`)
	require.Len(t, fieldsOf(plain), 2)
}
