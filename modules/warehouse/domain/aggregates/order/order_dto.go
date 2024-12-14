package order

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"time"
)

type CreateDTO struct {
	Type       string
	Status     string
	ProductIDs []uint
}

type UpdateDTO struct {
	Type       string
	Status     string
	ProductIDs []uint
}

func (d *CreateDTO) ToEntity() (*Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, id := range d.ProductIDs {
		items = append(items, Item{
			Position: position.Position{},
			Products: []product.Product{{ID: id}},
		})
	}
	return &Order{
		Type:      t,
		Status:    s,
		Items:     items,
		CreatedAt: time.Now(),
	}, nil
}

func (d *UpdateDTO) ToEntity(id uint) (*Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	var items []Item
	for _, productID := range d.ProductIDs {
		items = append(items, Item{
			Position: position.Position{},
			Products: []product.Product{{ID: productID}},
		})
	}
	return &Order{
		ID:        id,
		Type:      t,
		Status:    s,
		Items:     items,
		CreatedAt: time.Now(),
	}, nil
}
