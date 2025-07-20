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

	// Type assert to MultiLang
	ml, ok := value.Value().(models.MultiLang)
	if !ok {
		return templ.Raw("Invalid MultiLang")
	}

	return TableCell(ctx, ml)
}

// RenderDetails renders a MultiLang value for the details/view page
func (r *MultiLangRenderer) RenderDetails(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	if value == nil || value.IsZero() {
		return templ.Raw("")
	}

	// Type assert to MultiLang
	ml, ok := value.Value().(models.MultiLang)
	if !ok {
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
		// Type assert to MultiLang
		if v, ok := value.Value().(models.MultiLang); ok {
			ml = v
		} else {
			// Fallback: create empty MultiLang
			ml = models.NewMultiLangFromMap(map[string]string{})
		}
	}

	return FormInputWithJS(ctx, field, ml)
}
