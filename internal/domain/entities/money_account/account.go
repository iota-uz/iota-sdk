package moneyAccount

import (
	"github.com/go-playground/validator/v10"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var (
	validate = validator.New(validator.WithRequiredStructEnabled())
)

type Account struct {
	ID            uint
	Name          string
	AccountNumber string
	Description   string
	Balance       float64
	CurrencyCode  string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateDTO struct {
	Name          string  `validate:"required"`
	Balance       float64 `validate:"required,gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"required,len=3"`
	Description   string
}

type UpdateDTO struct {
	Name          string  `validate:"required,lte=255"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"required,len=3"`
	Description   string
}

// TODO: Add validations

func (p *CreateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() *Account {
	return &Account{}
}

// TODO: Add localization

func (p *Account) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Error()
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) Ok(l *i18n.Localizer) (map[string]string, bool) {
	errors := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errors, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		errors[_err.Field()] = _err.Error()
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity(id uint) *Account {
	return &Account{
		ID:            id,
		Name:          p.Name,
		AccountNumber: p.AccountNumber,
		Balance:       p.Balance,
		CurrencyCode:  p.CurrencyCode,
		Description:   p.Description,
	}
}
