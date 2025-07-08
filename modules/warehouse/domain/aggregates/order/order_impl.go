package order

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

// --- Implementation ---

func New(orderType Type, opts ...Option) Order {
	o := &order{
		id:        0,
		tenantID:  uuid.Nil,
		_type:     orderType,
		status:    Pending,
		items:     make([]Item, 0),
		createdAt: time.Now(),
		events:    make([]interface{}, 0),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type order struct {
	id        uint
	tenantID  uuid.UUID
	_type     Type
	status    Status
	items     []Item
	createdAt time.Time
	events    []interface{}
}

func (o *order) ID() uint {
	return o.id
}

func (o *order) TenantID() uuid.UUID {
	return o.tenantID
}

func (o *order) Type() Type {
	return o._type
}

func (o *order) Status() Status {
	return o.status
}

func (o *order) Items() []Item {
	return o.items
}

func (o *order) CreatedAt() time.Time {
	return o.createdAt
}

func (o *order) Events() []interface{} {
	return o.events
}

func (o *order) SetTenantID(tenantID uuid.UUID) Order {
	result := *o
	result.tenantID = tenantID
	return &result
}

func (o *order) AddItem(position position.Position, products ...product.Product) (Order, error) {
	for _, p := range products {
		if p.Status() == product.Shipped {
			return nil, NewErrProductIsShipped(p.Status())
		}
	}

	result := *o
	result.items = append(result.items, &item{
		position: position,
		products: products,
	})
	return &result, nil
}

func (o *order) Complete() (Order, error) {
	if o.status == Complete {
		return nil, NewErrOrderIsComplete(o.status)
	}

	result := *o
	result.status = Complete

	// Note: Product status changes should be handled by the domain service
	// that coordinates between Order and Product aggregates

	return &result, nil
}

type item struct {
	position position.Position
	products []product.Product
}

func (i *item) Position() position.Position {
	return i.position
}

func (i *item) Products() []product.Product {
	return i.products
}

func (i *item) Quantity() int {
	return len(i.products)
}
