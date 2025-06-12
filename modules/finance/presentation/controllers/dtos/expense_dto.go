package dtos

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type ExpenseCreateDTO struct {
	Amount           float64 `validate:"required,gt=0"`
	AccountID        string  `validate:"required,uuid"`
	CategoryID       string  `validate:"required,uuid"`
	Comment          string
	AccountingPeriod shared.DateOnly `validate:"required"`
	Date             shared.DateOnly `validate:"required"`
}

type ExpenseUpdateDTO struct {
	Amount           float64 `validate:"omitempty,gt=0"`
	AccountID        string  `validate:"omitempty,uuid"`
	CategoryID       string  `validate:"omitempty,uuid"`
	Comment          string
	AccountingPeriod shared.DateOnly
	Date             shared.DateOnly
}

func (d *ExpenseCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
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

func (d *ExpenseCreateDTO) ToEntity(tenantID uuid.UUID) (expense.Expense, error) {
	accountID, err := uuid.Parse(d.AccountID)
	if err != nil {
		return nil, fmt.Errorf("invalid account ID: %w", err)
	}

	categoryID, err := uuid.Parse(d.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("invalid category ID: %w", err)
	}

	// Create Money object from float amount, assuming USD as default
	amount := money.NewFromFloat(d.Amount, "USD")

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")
	account := moneyAccount.New("", defaultBalance, moneyAccount.WithID(accountID))
	expenseCategory := category.New(
		"", // name - will be populated when fetched from DB
		category.WithID(categoryID),
	)

	return expense.New(
		amount,
		account,
		expenseCategory,
		time.Time(d.Date),
		expense.WithComment(d.Comment),
		expense.WithAccountingPeriod(time.Time(d.AccountingPeriod)),
		expense.WithTenantID(tenantID),
	), nil
}

func (d *ExpenseUpdateDTO) Apply(entity expense.Expense, cat category.ExpenseCategory) (expense.Expense, error) {
	var accountID uuid.UUID
	var err error
	if d.AccountID != "" {
		accountID, err = uuid.Parse(d.AccountID)
		if err != nil {
			return nil, fmt.Errorf("invalid account ID: %w", err)
		}
	}

	// Create Money object from float amount if provided
	var amount *money.Money
	if d.Amount > 0 {
		amount = money.NewFromFloat(d.Amount, "USD")
	} else {
		amount = entity.Amount()
	}

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")
	entity = entity.
		SetAccount(moneyAccount.New("", defaultBalance, moneyAccount.WithID(accountID))).
		SetCategory(cat).
		SetComment(d.Comment).
		SetAmount(amount).
		SetDate(time.Time(d.Date)).
		SetAccountingPeriod(time.Time(d.AccountingPeriod))
	return entity, nil
}
