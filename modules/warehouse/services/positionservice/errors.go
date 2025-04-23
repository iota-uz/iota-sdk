package positionservice

import (
	"fmt"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func NewErrInvalidCell(col string, row uint) *ErrInvalidCell {
	return &ErrInvalidCell{
		BaseError: serrors.BaseError{
			Code:    "ERR_INVALID_CELL",
			Message: "Invalid cell found",
		},
		Col: col,
		Row: row,
	}
}

type ErrInvalidCell struct {
	serrors.BaseError
	Col string
	Row uint
}

func (e *ErrInvalidCell) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Errors.%s", e.Code),
		},
		TemplateData: map[string]interface{}{
			"Row": e.Row,
			"Col": e.Col,
		},
	})
}
