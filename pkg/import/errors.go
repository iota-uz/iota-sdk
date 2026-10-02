package importpkg

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type DefaultErrorFactory struct{}

func NewDefaultErrorFactory() *DefaultErrorFactory { return &DefaultErrorFactory{} }
func (f *DefaultErrorFactory) NewInvalidCellError(col string, row uint) error {
	return &InvalidCellError{detail: serrors.NewInvalid("Invalid cell found").WithReason("ERR_INVALID_CELL").WithPublic(serrors.Message{ID: "ERR_INVALID_CELL", Args: map[string]serrors.Value{"Row": serrors.Number(int64(row)), "Col": serrors.Text(col)}}), Col: col, Row: row}
}
func (f *DefaultErrorFactory) NewValidationError(col, value string, rowNum uint, message string) error {
	return &ValidationError{detail: serrors.NewInvalid(message).WithReason("ERR_VALIDATION").WithPublic(serrors.Message{ID: "Error.ERR_VALIDATION", Args: map[string]serrors.Value{"Col": serrors.Text(col), "Value": serrors.Text(value), "RowNum": serrors.Number(int64(rowNum)), "Message": serrors.Text(message)}}), Col: col, Value: value, RowNum: rowNum}
}

type InvalidCellError struct {
	detail *serrors.Error
	Col    string
	Row    uint
}

func (e *InvalidCellError) Unwrap() error                     { return e.detail }
func (e *InvalidCellError) Localize(l *i18n.Localizer) string { return serrors.Public(e, l).Message }

type ValidationError struct {
	detail *serrors.Error
	Col    string
	Value  string
	RowNum uint
}

func (e *ValidationError) Unwrap() error                     { return e.detail }
func (e *ValidationError) Localize(l *i18n.Localizer) string { return serrors.Public(e, l).Message }
func (e *InvalidCellError) Error() string                    { return e.detail.Error() }
func (e *ValidationError) Error() string                     { return e.detail.Error() }
