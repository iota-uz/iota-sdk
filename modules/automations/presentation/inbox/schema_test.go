package inbox

import (
	"encoding/json"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFieldsOfKeepsOrderAndKinds(t *testing.T) {
	t.Parallel()
	fields := fieldsOf(json.RawMessage(`{"type":"object","required":["priority"],"properties":{
		"note":{"type":"string","maxLength":100},"amount":{"type":"number","title":"Amount"},"count":{"type":"integer"},
		"urgent":{"type":"boolean"},"priority":{"type":"string","enum":["low","high"]},"extra":{"type":"object"},"text":{"type":"string"}}}`))
	require.Len(t, fields, 7)
	var names, kinds []string
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
