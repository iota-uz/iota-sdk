package order

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"time"
)

type Order interface {
	ID() uint
	Type() Type
	Status() Status
	Items() []Item
	CreatedAt() time.Time

	SetID(id uint)

	AddItem(position *position.Position, products ...*product.Product) error
	Complete() error
}

type Item interface {
	Position() *position.Position
	Products() []*product.Product
	Quantity() int
}
