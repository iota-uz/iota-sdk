package multilang

import (
	"context"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
)

// MultiLangRenderer implements crud.FieldRenderer for MultiLang fields
type MultiLangRenderer struct{}

// NewMultiLangRenderer creates a new MultiLang field renderer
func NewMultiLangRenderer() *MultiLangRenderer {
	return &MultiLangRenderer{}
}

// RenderTableCell renders a MultiLang value for display in table rows
func (r *MultiLangRenderer) RenderTableCell(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	if value == nil || value.IsZero() {
		return templ.Raw("")
	}

	ml := getMultiLangFromValue(value)
	if ml == nil {
		return templ.Raw("Invalid MultiLang")
	}

	return TableCell(ctx, ml)
}

// RenderDetails renders a MultiLang value for the details/view page
func (r *MultiLangRenderer) RenderDetails(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	if value == nil || value.IsZero() {
		return templ.Raw("")
	}

	ml := getMultiLangFromValue(value)
	if ml == nil {
		return templ.Raw("Invalid MultiLang")
	}

	return DetailsView(ctx, ml)
}

// RenderFormControl renders a MultiLang field as an editable form input
func (r *MultiLangRenderer) RenderFormControl(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	var ml models.MultiLang

	// Handle nil or zero values
	if value == nil || value.IsZero() {
		ml = models.NewMultiLangFromMap(map[string]string{})
	} else {
		ml = getMultiLangFromValue(value)
		if ml == nil {
			// Fallback: create empty MultiLang
			ml = models.NewMultiLangFromMap(map[string]string{})
		}
	}

	return FormInputWithJS(ctx, field, ml)
}

// getMultiLangFromValue extracts MultiLang from a crud.FieldValue
// Handles both direct MultiLang objects and JSON strings from database
func getMultiLangFromValue(value crud.FieldValue) models.MultiLang {
	if value == nil {
		return nil
	}

	// Try direct MultiLang type assertion first
	if ml, ok := value.Value().(models.MultiLang); ok {
		return ml
	}

	// Try JSON string from database
	if jsonStr, ok := value.Value().(string); ok && jsonStr != "" {
		if ml, err := models.MultiLangFromString(jsonStr); err == nil {
			return ml
		}
	}

	// Try map[string]interface{} from database JSON parsing
	if mapVal, ok := value.Value().(map[string]interface{}); ok {
		stringMap := make(map[string]string)
		for k, v := range mapVal {
			if str, ok := v.(string); ok {
				stringMap[k] = str
			}
		}
		if len(stringMap) > 0 {
			return models.NewMultiLangFromMap(stringMap)
		}
	}

	return nil
}
