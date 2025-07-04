package currency

import (
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type Currency struct {
	Code      Code
	Name      string
	Symbol    Symbol
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateDTO struct {
	Code   string `validate:"required"`
	Name   string `validate:"required"`
	Symbol string `validate:"required"`
}

type UpdateDTO struct {
	Code   string `validate:"len=3"`
	Name   string
	Symbol string
}

func (p *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() (*Currency, error) {
	c, err := NewCode(p.Code)
	if err != nil {
		return nil, err
	}
	s, err := NewSymbol(p.Symbol)
	if err != nil {
		return nil, err
	}
	return &Currency{
		Code:   c,
		Name:   p.Name,
		Symbol: s,
	}, nil
}

func (p *Currency) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity() (*Currency, error) {
	c, err := NewCode(p.Code)
	if err != nil {
		return nil, err
	}
	s, err := NewSymbol(p.Symbol)
	if err != nil {
		return nil, err
	}
	return &Currency{
		Code:   c,
		Name:   p.Name,
		Symbol: s,
	}, nil
}
