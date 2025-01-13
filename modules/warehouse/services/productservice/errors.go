package productservice

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type ErrDuplicateRfid struct {
	serrors.BaseError
	Rfid string
}

func NewErrDuplicateRfid(rfid string) *ErrDuplicateRfid {
	return &ErrDuplicateRfid{
		BaseError: serrors.BaseError{
			Code:    "ERR_DUPLICATE_RFID",
			Message: fmt.Sprintf("Rfid %s already exists", rfid),
		},
		Rfid: rfid,
	}
}

func (e *ErrDuplicateRfid) Localize(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Errors.%s", e.Code),
		},
		TemplateData: map[string]interface{}{
			"Rfid": e.Rfid,
		},
	})
}
