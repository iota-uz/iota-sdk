package project

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	Name        string `validate:"required"`
	Description string
}

type UpdateDTO struct {
	Name        string
	Description string
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

func (d *CreateDTO) ToEntity() *Project {
	return &Project{
		ID:          0,
		Name:        d.Name,
		Description: d.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

func (d *UpdateDTO) ToEntity(id uint) *Project {
	return &Project{
		ID:          id,
		Name:        d.Name,
		Description: d.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}
