package viewmodels

import (
	"fmt"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Product struct {
	ID         string
	PositionID string
	Position   *Position
	Rfid       string
	Status     string
	CreatedAt  string
	UpdatedAt  string
}

func (p *Product) LocalizedStatus(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("Products.Statuses.%s", p.Status),
		},
	})
}
