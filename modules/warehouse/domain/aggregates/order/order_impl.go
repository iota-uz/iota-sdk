package order

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
)

func New(orderType Type, status Status) Order {
	return &orderImpl{
		_type:     orderType,
		status:    status,
		items:     make([]Item, 0),
		createdAt: time.Now(),
	}
}

func NewWithID(id uint, orderType Type, status Status, createdAt time.Time) Order {
	return &orderImpl{
		id:        id,
		_type:     orderType,
		status:    status,
		items:     make([]Item, 0),
		createdAt: createdAt,
	}
}

type orderImpl struct {
	id        uint
	_type     Type
	status    Status
	items     []Item
	createdAt time.Time
}

func (o *orderImpl) SetID(id uint) {
	o.id = id
}

func (o *orderImpl) ID() uint {
	return o.id
}

func (o *orderImpl) Type() Type {
	return o._type
}

func (o *orderImpl) Status() Status {
	return o.status
}

func (o *orderImpl) Items() []Item {
	return o.items
}

func (o *orderImpl) CreatedAt() time.Time {
	return o.createdAt
}

func (o *orderImpl) AddItem(position *position.Position, products ...*product.Product) error {
	for _, p := range products {
		if p.Status == product.Shipped {
			return NewErrProductIsShipped(p.Status)
		}
	}
	o.items = append(o.items, &itemImpl{
		position: position,
		products: products,
	})
	return nil
}

func (o *orderImpl) Complete() error {
	if o.status == Complete {
		return NewErrOrderIsComplete(o.status)
	}
	var status product.Status
	if o._type == TypeIn {
		status = product.InStock
	} else {
		status = product.Approved
	}
	for _, item := range o.items {
		for _, p := range item.Products() {
			p.Status = status
		}
	}
	o.status = Complete
	return nil
}

type itemImpl struct {
	position *position.Position
	products []*product.Product
}

func (i *itemImpl) Position() *position.Position {
	return i.position
}

func (i *itemImpl) Products() []*product.Product {
	return i.products
}

func (i *itemImpl) Quantity() int {
	return len(i.products)
}
