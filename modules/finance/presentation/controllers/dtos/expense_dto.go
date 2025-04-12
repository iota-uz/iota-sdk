package dtos

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

type ExpenseCreateDTO struct {
	Amount           float64
	AccountID        uint
	CategoryID       uint
	Comment          string
	AccountingPeriod time.Time
	Date             time.Time
}

type ExpenseUpdateDTO struct {
	Amount           float64
	AccountID        uint
	CategoryID       uint
	Comment          string
	AccountingPeriod time.Time
	Date             time.Time
}

func (d *ExpenseCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}

	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Expenses.Single.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *ExpenseUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := composables.UseLocalizer(ctx)
	if !ok {
		panic(composables.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(d)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Expenses.Single.%s", err.Field()),
		})
		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *ExpenseCreateDTO) ToEntity() (expense.Expense, error) {
	account := moneyAccount.Account{ID: d.AccountID}
	expenseCategory := category.New(
		"",  // name - will be populated when fetched from DB
		0,   // amount - will be populated when fetched from DB
		nil, // currency - will be populated when fetched from DB
		category.WithID(d.CategoryID),
	)

	return expense.New(
		d.Amount,
		account,
		expenseCategory,
		d.Date,
		expense.WithComment(d.Comment),
		expense.WithAccountingPeriod(d.AccountingPeriod),
	), nil
}

func (d *ExpenseUpdateDTO) ToEntity(id uint) (expense.Expense, error) {
	account := moneyAccount.Account{ID: d.AccountID}
	expenseCategory := category.New(
		"",  // name - will be populated when fetched from DB
		0,   // amount - will be populated when fetched from DB
		nil, // currency - will be populated when fetched from DB
		category.WithID(d.CategoryID),
	)

	return expense.New(
		d.Amount,
		account,
		expenseCategory,
		d.Date,
		expense.WithID(id),
		expense.WithComment(d.Comment),
		expense.WithAccountingPeriod(d.AccountingPeriod),
	), nil
}
