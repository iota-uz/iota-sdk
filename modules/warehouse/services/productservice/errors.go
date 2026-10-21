package productservice

import (
	"fmt"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type DuplicateRfidError struct {
	detail *serrors.Error
	Rfid   string
}

func NewErrDuplicateRfid(rfid string) *DuplicateRfidError {
	return &DuplicateRfidError{detail: serrors.NewAlreadyExists(fmt.Sprintf("Rfid %s already exists", rfid)).WithReason("ERR_DUPLICATE_RFID").WithPublic(serrors.Message{ID: "Errors.ERR_DUPLICATE_RFID", Args: map[string]serrors.Value{"Rfid": serrors.Text(rfid)}}), Rfid: rfid}
}
func (e *DuplicateRfidError) Unwrap() error                     { return e.detail }
func (e *DuplicateRfidError) Localize(l *i18n.Localizer) string { return serrors.Public(e, l).Message }
func (e *DuplicateRfidError) Error() string                     { return e.detail.Error() }
