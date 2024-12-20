package order

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
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

func (d *CreateDTO) ToEntity() (Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	entity := New(t, s)
	for _, id := range d.ProductIDs {
		if err := entity.AddItem(position.Position{}, &product.Product{ID: id}); err != nil {
			return nil, err
		}
	}
	return entity, nil
}

func (d *UpdateDTO) ToEntity(id uint) (Order, error) {
	t, err := NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	entity := New(t, s)
	for _, productID := range d.ProductIDs {
		if err := entity.AddItem(position.Position{}, &product.Product{ID: productID}); err != nil {
			return nil, err
		}
	}
	entity.SetID(id)
	return entity, nil
}
