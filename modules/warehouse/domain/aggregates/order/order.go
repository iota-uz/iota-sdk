package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

type Option func(o *order)

// --- Option setters ---

func WithID(id uint) Option {
	return func(o *order) {
		o.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(o *order) {
		o.tenantID = tenantID
	}
}

func WithStatus(status Status) Option {
	return func(o *order) {
		o.status = status
	}
}

func WithItems(items []Item) Option {
	return func(o *order) {
		o.items = items
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(o *order) {
		o.createdAt = createdAt
	}
}

// --- Interfaces ---

type Order interface {
	ID() uint
	TenantID() uuid.UUID
	Type() Type
	Status() Status
	Items() []Item
	CreatedAt() time.Time

	Events() []interface{}

	SetTenantID(tenantID uuid.UUID) Order
	AddItem(position position.Position, products ...product.Product) (Order, error)
	Complete() (Order, error)
}

type Item interface {
	Position() position.Position
	Products() []product.Product
	Quantity() int
}
