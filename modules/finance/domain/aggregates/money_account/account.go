package moneyaccount

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"time"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type Account struct {
	ID            uint
	Name          string
	AccountNumber string
	Description   string
	Balance       float64
	Currency      currency.Currency
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (a *Account) InitialTransaction() *transaction.Transaction {
	return transaction.NewDeposit(
		a.Balance,
		string(a.Currency.Code),
		0,
		a.ID,
		a.CreatedAt,
		a.CreatedAt,
		"",
	)
}

func (a *Account) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(a)
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

func (p *UpdateDTO) ToEntity(id uint) (*Account, error) {
	code, err := currency.NewCode(p.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &Account{
		ID:            id,
		Name:          p.Name,
		AccountNumber: p.AccountNumber,
		Balance:       p.Balance,
		Currency:      currency.Currency{Code: code},
		Description:   p.Description,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}, nil
}
