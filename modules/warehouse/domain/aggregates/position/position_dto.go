package position

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"time"
)

type CreateDTO struct {
	Title   string
	Barcode string
	UnitID  uint
}

type UpdateDTO struct {
	Title   string
	Barcode string
	UnitID  uint
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

func (d *CreateDTO) ToEntity() (*Position, error) {
	return &Position{
		ID:        0,
		Title:     d.Title,
		Barcode:   d.Barcode,
		UnitID:    d.UnitID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (d *UpdateDTO) ToEntity(id uint) (*Position, error) {
	return &Position{
		ID:        id,
		Title:     d.Title,
		Barcode:   d.Barcode,
		UnitID:    d.UnitID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}