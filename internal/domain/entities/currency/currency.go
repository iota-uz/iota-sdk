package currency

import (
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
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

func (p *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	// TODO: Add validations
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

func (p *Currency) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	// TODO: Add validations
	return errors, len(errors) == 0
}

func (p *UpdateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	// TODO: Add validations
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
