package tab

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-sdk/pkg/constants"
)

type CreateDTO struct {
	Href     string `validate:"required"`
	UserID   uint
	Position uint
}

type UpdateDTO struct {
	Href     string `validate:"required"`
	Position uint
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

func (d *CreateDTO) ToEntity() (*Tab, error) {
	return &Tab{
		ID:       0,
		Href:     d.Href,
		Position: d.Position,
		UserID:   d.UserID,
	}, nil
}

func (d *UpdateDTO) ToEntity(id uint) (*Tab, error) {
	return &Tab{
		ID:       id,
		Href:     d.Href,
		Position: d.Position,
	}, nil
}
