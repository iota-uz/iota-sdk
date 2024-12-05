package dtos

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"time"
)

type CreateOrderDTO struct {
	Type       string `validate:"required"`
	Status     string `validate:"required"`
	ProductIDs []uint `validate:"required"`
}

type UpdateOrderDTO struct {
	Type       string
	Status     string
	ProductIDs []uint
}

func (d *CreateOrderDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = err.Translate(l)
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *UpdateOrderDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (d *CreateOrderDTO) ToEntity() (*order.Order, error) {
	t, err := order.NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := order.NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	var products []*product.Product
	for _, id := range d.ProductIDs {
		products = append(products, &product.Product{ID: id})
	}
	return &order.Order{
		ID:        0,
		Type:      t,
		Status:    s,
		Products:  products,
		CreatedAt: time.Now(),
	}, nil
}

func (d *UpdateOrderDTO) ToEntity(id uint) (*order.Order, error) {
	t, err := order.NewType(d.Type)
	if err != nil {
		return nil, err
	}
	s, err := order.NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	var products []*product.Product
	for _, id := range d.ProductIDs {
		products = append(products, &product.Product{ID: id})
	}
	return &order.Order{
		ID:        id,
		Type:      t,
		Status:    s,
		Products:  products,
		CreatedAt: time.Now(),
	}, nil
}
