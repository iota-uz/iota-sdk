package productservice

import (
	"fmt"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type DuplicateRfidError struct {
	serrors.BaseError
	Rfid string
}

func NewErrDuplicateRfid(rfid string) *DuplicateRfidError {
	return &DuplicateRfidError{
		BaseError: serrors.BaseError{
			Code:    "ERR_DUPLICATE_RFID",
			Message: fmt.Sprintf("Rfid %s already exists", rfid),
		},
		Rfid: rfid,
	}
}

func (e *DuplicateRfidError) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Errors.%s", e.Code),
		},
		TemplateData: map[string]interface{}{
			"Rfid": e.Rfid,
		},
	})
}
