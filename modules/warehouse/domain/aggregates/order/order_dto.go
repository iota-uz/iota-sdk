package order

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
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
	entity := New(t, WithStatus(s))
	for _, id := range d.ProductIDs {
		// Create temporary position and product instances for the DTO
		// Note: In a real implementation, these would be fetched from repositories
		pos := position.New("", "", position.WithID(1))
		prod := product.New("", product.InStock, product.WithID(id))
		entity, err = entity.AddItem(pos, prod)
		if err != nil {
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
	entity := New(t, WithID(id), WithStatus(s))
	for _, productID := range d.ProductIDs {
		// Create temporary position and product instances for the DTO
		// Note: In a real implementation, these would be fetched from repositories
		pos := position.New("", "", position.WithID(1))
		prod := product.New("", product.InStock, product.WithID(productID))
		entity, err = entity.AddItem(pos, prod)
		if err != nil {
			return nil, err
		}
	}
	return entity, nil
}
