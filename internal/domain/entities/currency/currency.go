package currency

import (
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type Currency struct {
	Code      string
	Name      string
	Symbol    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateDTO struct {
	Code   string `schema:"amount,required"`
	Name   string `schema:"amount_currency_id,required"`
	Symbol string `schema:"money_account_id,required"`
}

type UpdateDTO struct {
	Code   string `schema:"amount"`
	Name   string `schema:"amount_currency_id"`
	Symbol string `schema:"money_account_id"`
}

func (p *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	// TODO: Add validations
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() *Currency {
	return &Currency{
		Code:   p.Code,
		Name:   p.Name,
		Symbol: p.Symbol,
	}
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

func (p *UpdateDTO) ToEntity() *Currency {
	return &Currency{
		Code:   p.Code,
		Name:   p.Name,
		Symbol: p.Symbol,
	}
}
