package multilang

import (
	"context"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

// MultiLangRenderer implements crud.FieldRenderer for MultiLang fields
type MultiLangRenderer struct {
	schema crud.Schema[any]
}

// NewMultiLangRenderer creates a new MultiLang field renderer
func NewMultiLangRenderer() *MultiLangRenderer {
	return &MultiLangRenderer{}
}

// NewMultiLangRendererWithSchema creates a new MultiLang field renderer with schema
func NewMultiLangRendererWithSchema[TEntity any](schema crud.Schema[TEntity]) *MultiLangRenderer {
	// Type assertion to store as any - this is safe since we only use schema.Name()
	anySchema := schema.(crud.Schema[any])
	return &MultiLangRenderer{schema: anySchema}
}

// RenderTableCell renders a MultiLang value for display in table rows
func (r *MultiLangRenderer) RenderTableCell(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	if value == nil || value.IsZero() {
		return templ.Raw("")
	}

	ml := getMultiLangFromValue(value)
	if ml == nil {
		errorMsg := r.localizeWithDefault(ctx, "multilang.invalid_format", "Invalid MultiLang")
		return templ.Raw(errorMsg)
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
		errorMsg := r.localizeWithDefault(ctx, "multilang.invalid_format", "Invalid MultiLang")
		return templ.Raw(errorMsg)
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

	// Get localized label using the same pattern as crud_controller
	label := r.getLocalizedFieldLabel(ctx, field)

	return FormInputWithJSAndLabel(ctx, field, ml, label)
}

// RenderFormControlWithLabel renders a MultiLang field with custom label
func (r *MultiLangRenderer) RenderFormControlWithLabel(ctx context.Context, field crud.Field, value crud.FieldValue, label string) templ.Component {
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

	return FormInputWithJSAndLabel(ctx, field, ml, label)
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

// localizeWithDefault localizes a message with a fallback default
func (r *MultiLangRenderer) localizeWithDefault(ctx context.Context, messageID string, defaultMessage string) string {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		return defaultMessage
	}

	result, err := l.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
		DefaultMessage: &i18n.Message{
			ID:    messageID,
			Other: defaultMessage,
		},
	})
	if err != nil {
		return defaultMessage
	}
	return result
}

// getLocalizedFieldLabel returns the localized field label using the same pattern as crud_controller
func (r *MultiLangRenderer) getLocalizedFieldLabel(ctx context.Context, field crud.Field) string {
	// Localize field label using custom key if provided, otherwise use default pattern
	localizationKey := field.LocalizationKey()
	if localizationKey == "" && r.schema != nil {
		localizationKey = r.schema.Name() + ".Fields." + field.Name()
	} else if localizationKey == "" {
		// Fallback when schema is not available
		localizationKey = "Fields." + field.Name()
	}

	fieldLabel := r.localizeWithDefault(ctx, localizationKey, field.Name())
	return fieldLabel
}
