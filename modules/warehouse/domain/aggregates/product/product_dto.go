package product

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"time"
)

type CreateDTO struct {
	PositionID uint
	Rfid       string
	Status     string
}

type UpdateDTO struct {
	PositionID uint
	Rfid       string
	Status     string
}

type CreateProductsFromTagsDTO struct {
	Tags       []string
	PositionID uint
}

func (d *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
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

func (d *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
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

func (d *CreateDTO) ToEntity() (*Product, error) {
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	return &Product{
		ID:         0,
		PositionID: d.PositionID,
		Rfid:       d.Rfid,
		Status:     s,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (d *UpdateDTO) ToEntity(id uint) (*Product, error) {
	s, err := NewStatus(d.Status)
	if err != nil {
		return nil, err
	}
	return &Product{
		ID:         id,
		PositionID: d.PositionID,
		Rfid:       d.Rfid,
		Status:     s,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}
