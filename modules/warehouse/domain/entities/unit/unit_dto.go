package unit

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"time"
)

type CreateDTO struct {
	Title      string `validate:"required"`
	ShortTitle string `validate:"required"`
}

type UpdateDTO struct {
	Title      string
	ShortTitle string
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

func (d *CreateDTO) ToEntity() (*Unit, error) {
	return &Unit{
		ID:         0,
		Title:      d.Title,
		ShortTitle: d.ShortTitle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}

func (d *UpdateDTO) ToEntity(id uint) (*Unit, error) {
	return &Unit{
		ID:         id,
		Title:      d.Title,
		ShortTitle: d.ShortTitle,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}, nil
}
