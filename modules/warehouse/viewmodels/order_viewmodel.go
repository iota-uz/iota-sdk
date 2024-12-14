package viewmodels

import (
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Order struct {
	ID        string
	Type      string
	Status    string
	CreatedAt string
	UpdatedAt string
}

func (o *Order) LocalizedStatus(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseOrders.Single.Statuses.%s", o.Status),
		},
	})
}

func (o *Order) LocalizedType(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseOrders.Single.Types.%s", o.Type),
		},
	})
}

type OrderItem struct {
	Position Position
	Quantity string
}
