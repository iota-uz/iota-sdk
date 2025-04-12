package expense

import (
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"time"
)

type CreateDTO struct {
	Amount           float64
	AccountID        uint
	CategoryID       uint
	Comment          string
	AccountingPeriod time.Time
	Date             time.Time
}

type UpdateDTO struct {
	Amount           float64
	AccountID        uint
	CategoryID       uint
	Comment          string
	AccountingPeriod time.Time
	Date             time.Time
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

func (d *CreateDTO) ToEntity() (Expense, error) {
	account := moneyAccount.Account{ID: d.AccountID}
	expenseCategory := category.NewWithID(
		d.CategoryID,
		"",
		"",
		0,
		nil,
		time.Now(),
		time.Now(),
	)

	return New(
		d.Amount,
		account,
		expenseCategory,
		d.Date,
		WithComment(d.Comment),
		WithAccountingPeriod(d.AccountingPeriod),
	), nil
}

func (d *UpdateDTO) ToEntity(id uint) (Expense, error) {
	account := moneyAccount.Account{ID: d.AccountID}
	expenseCategory := category.NewWithID(
		d.CategoryID,
		"",
		"",
		0,
		nil,
		time.Now(),
		time.Now(),
	)

	return New(
		d.Amount,
		account,
		expenseCategory,
		d.Date,
		WithID(id),
		WithComment(d.Comment),
		WithAccountingPeriod(d.AccountingPeriod),
	), nil
}
