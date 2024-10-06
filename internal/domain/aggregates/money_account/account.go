package moneyAccount

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-agency/iota-erp/internal/domain/entities/currency"
	"github.com/iota-agency/iota-erp/pkg/middleware"
	"time"
)

type Account struct {
	Id            uint
	Name          string
	AccountNumber string
	Description   string
	Balance       float64
	Currency      currency.Currency
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
	Name          string  `validate:"lte=255"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"len=3"`
	Description   string
}

// TODO: Add validations

func (p *CreateDTO) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := middleware.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *CreateDTO) ToEntity() *Account {
	return &Account{}
}

// TODO: Add localization

func (p *Account) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := middleware.Validate.Struct(p)
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
	errs := middleware.Validate.Struct(p)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}

func (p *UpdateDTO) ToEntity(id uint) (*Account, error) {
	code, err := currency.NewCode(p.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &Account{
		Id:            id,
		Name:          p.Name,
		AccountNumber: p.AccountNumber,
		Balance:       p.Balance,
		Currency:      currency.Currency{Code: code},
		Description:   p.Description,
	}, nil
}
