package viewmodels

import (
	"fmt"
	"strconv"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Order struct {
	ID        string
	Type      string
	Status    string
	Items     []OrderItem
	CreatedAt string
	UpdatedAt string
}

func (o *Order) LocalizedStatus(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseOrders.Statuses.%s", o.Status),
		},
	})
}

func (o *Order) LocalizedType(l *i18n.Localizer) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		DefaultMessage: &i18n.Message{
			ID: fmt.Sprintf("WarehouseOrders.Types.%s", o.Type),
		},
	})
}

func (o *Order) DistinctPositions() string {
	return strconv.Itoa(len(o.Items))
}

func (o *Order) TotalProducts() string {
	var totalProducts int
	for _, pos := range o.Items {
		totalProducts += len(pos.Products)
	}
	return strconv.Itoa(totalProducts)
}

type OrderItem struct {
	Position Position
	Products []Product
	InStock  string
}

func (oi *OrderItem) Quantity() string {
	return strconv.Itoa(len(oi.Products))
}
