package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type MoneyAccountCreateDTO struct {
	Name          string  `validate:"required"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"required,len=3"`
	Description   string
}

type MoneyAccountUpdateDTO struct {
	Name          string  `validate:"lte=255"`
	Balance       float64 `validate:"gte=0"`
	AccountNumber string
	CurrencyCode  string `validate:"len=3"`
	Description   string
}

func (p *MoneyAccountCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("MoneyAccounts.Single.%s", err.Field()),
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

func (p *MoneyAccountCreateDTO) ToEntity(tenantID uuid.UUID) (moneyaccount.Account, error) {
	// Create Money object from balance and currency
	balance := money.NewFromFloat(p.Balance, p.CurrencyCode)

	return moneyaccount.New(
		p.Name,
		balance,
		moneyaccount.WithTenantID(tenantID),
		moneyaccount.WithAccountNumber(p.AccountNumber),
		moneyaccount.WithDescription(p.Description),
	), nil
}

func (p *MoneyAccountUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(p)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("MoneyAccounts.Single.%s", err.Field()),
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

func (p *MoneyAccountUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID) (moneyaccount.Account, error) {
	// Create Money object from balance and currency
	balance := money.NewFromFloat(p.Balance, p.CurrencyCode)

	return moneyaccount.New(
		p.Name,
		balance,
		moneyaccount.WithID(id),
		moneyaccount.WithTenantID(tenantID),
		moneyaccount.WithAccountNumber(p.AccountNumber),
		moneyaccount.WithDescription(p.Description),
	), nil
}

func (p *MoneyAccountUpdateDTO) Apply(existing moneyaccount.Account) (moneyaccount.Account, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	// Create Money object from balance and currency if provided
	var balance *money.Money
	if p.CurrencyCode != "" {
		balance = money.NewFromFloat(p.Balance, p.CurrencyCode)
	} else {
		balance = existing.Balance()
	}

	existing = existing.
		UpdateName(p.Name).
		UpdateBalance(balance).
		UpdateAccountNumber(p.AccountNumber).
		UpdateDescription(p.Description)

	return existing, nil
}
