package positionservice

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func NewErrInvalidCell(col string, row uint) *InvalidCellError {
	return &InvalidCellError{detail: serrors.NewInvalid("Invalid cell found").WithReason("ERR_INVALID_CELL").WithPublic(serrors.Message{ID: "Errors.ERR_INVALID_CELL", Args: map[string]serrors.Value{"Row": serrors.Number(int64(row)), "Col": serrors.Text(col)}}), Col: col, Row: row}
}

type InvalidCellError struct {
	detail *serrors.Error
	Col    string
	Row    uint
}

func (e *InvalidCellError) Unwrap() error                     { return e.detail }
func (e *InvalidCellError) Localize(l *i18n.Localizer) string { return serrors.Public(e, l).Message }
func (e *InvalidCellError) Error() string                     { return e.detail.Error() }
