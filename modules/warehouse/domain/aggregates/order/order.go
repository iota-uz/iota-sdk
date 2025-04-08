package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

type Order interface {
	ID() uint
	TenantID() uuid.UUID
	Type() Type
	Status() Status
	Items() []Item
	CreatedAt() time.Time

	SetID(id uint)
	SetTenantID(id uuid.UUID)

	AddItem(position *position.Position, products ...*product.Product) error
	Complete() error
}

type Item interface {
	Position() *position.Position
	Products() []*product.Product
	Quantity() int
}
