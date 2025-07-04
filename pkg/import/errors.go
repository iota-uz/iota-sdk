package importpkg

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// DefaultErrorFactory implements ErrorFactory
type DefaultErrorFactory struct{}

// NewDefaultErrorFactory creates a new error factory
func NewDefaultErrorFactory() *DefaultErrorFactory {
	return &DefaultErrorFactory{}
}

func (f *DefaultErrorFactory) NewInvalidCellError(col string, row uint) error {
	return &InvalidCellError{
		BaseError: serrors.BaseError{
			Code:    "ERR_INVALID_CELL",
			Message: "Invalid cell found",
		},
		Col: col,
		Row: row,
	}
}

func (f *DefaultErrorFactory) NewValidationError(col, value string, rowNum uint, message string) error {
	return &ValidationError{
		BaseError: serrors.BaseError{
			Code:    "ERR_VALIDATION",
			Message: message,
		},
		Col:    col,
		Value:  value,
		RowNum: rowNum,
	}
}

// InvalidCellError represents an error in a specific cell
type InvalidCellError struct {
	serrors.BaseError
	Col string
	Row uint
}

func (e *InvalidCellError) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: e.Code,
		},
		TemplateData: map[string]interface{}{
			"Row": e.Row,
			"Col": e.Col,
		},
	})
}

// ValidationError represents a validation error with context
type ValidationError struct {
	serrors.BaseError
	Col    string
	Value  string
	RowNum uint
}

func (e *ValidationError) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: "Error.ERR_VALIDATION",
		},
		TemplateData: map[string]interface{}{
			"Col":     e.Col,
			"Value":   e.Value,
			"RowNum":  e.RowNum,
			"Message": e.Message,
		},
	})
}
