package history

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// ChangeDisplay is the change of one object as a reader sees it.
type ChangeDisplay struct {
	Resource string `json:"resource"`
	ID       string `json:"id"`
	// Subject is the object's label, as in Item.Subjects.
	Subject string         `json:"subject"`
	Fields  []FieldDisplay `json:"fields"`
}

// FieldDisplay is the change of one field as a reader sees it.
type FieldDisplay struct {
	Field  string       `json:"field"`
	Label  string       `json:"label"`
	Before ValueDisplay `json:"before"`
	After  ValueDisplay `json:"after"`
}

// ValueDisplay is a field value as a reader sees it. Text is set for a present value; Code keeps the
// stored value when Text is its translation.
type ValueDisplay struct {
	State fielddiff.State `json:"state"`
	Text  string          `json:"text,omitempty"`
	Code  string          `json:"code,omitempty"`
}

func displayValue(ctx context.Context, field Field, v fielddiff.FieldValue) ValueDisplay {
	if v.State != fielddiff.StateValue {
		return ValueDisplay{State: v.State}
	}
	text := FormatValue(v)
	code, scalar := scalarCode(v)
	if !scalar {
		return ValueDisplay{State: v.State, Text: text}
	}
	if translated := translateValue(ctx, field, code); translated != "" && translated != code {
		return ValueDisplay{State: v.State, Text: translated, Code: code}
	}
	return ValueDisplay{State: v.State, Text: text}
}

// translateValue returns the reader-facing text of a stored code, or "" when the field declares none for it.
func translateValue(ctx context.Context, field Field, code string) string {
	if field.ValueLabel != nil {
		if text := strings.TrimSpace(field.ValueLabel(ctx, code)); text != "" {
			return text
		}
	}
	if field.ValueKeyPrefix != "" && code != "" {
		if text, ok := intl.T(ctx, field.ValueKeyPrefix+"."+code); ok && text != "" {
			return text
		}
	}
	return ""
}

// scalarCode is the code form of a string, number or boolean value.
func scalarCode(v fielddiff.FieldValue) (string, bool) {
	value, err := decode(v.Value)
	if err != nil {
		return "", false
	}
	switch value := value.(type) {
	case string:
		return value, true
	case bool, json.Number:
		return string(bytes.TrimSpace(v.Value)), true
	}
	return "", false
}

// FormatValue renders a present field value for reading: strings as they are, money as
// "<amount> <currency>", null as "—".
func FormatValue(v fielddiff.FieldValue) string {
	value, err := decode(v.Value)
	if err != nil {
		return string(v.Value)
	}
	switch value := value.(type) {
	case nil:
		return "—"
	case string:
		return value
	case bool, json.Number:
		return string(v.Value)
	case map[string]any:
		if amount, ok := value["amount"].(string); ok {
			currency, _ := value["currency"].(string)
			return amount + " " + currency
		}
	}
	return string(v.Value)
}

func decode(raw json.RawMessage) (any, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err := decoder.Decode(&value)
	return value, err
}
