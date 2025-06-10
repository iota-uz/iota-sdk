package dtos

import (
	"context"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

type PaymentCreateDTO struct {
	Amount           float64         `validate:"required,gt=0"`
	AccountID        uint            `validate:"required"`
	TransactionDate  shared.DateOnly `validate:"required"`
	AccountingPeriod shared.DateOnly `validate:"required"`
	CounterpartyID   uint            `validate:"required"`
	UserID           uint            `validate:"required"`
	Comment          string
}

type PaymentUpdateDTO struct {
	Amount           float64 `validate:"gt=0"`
	AccountID        uint
	CounterpartyID   uint
	TransactionDate  shared.DateOnly
	AccountingPeriod shared.DateOnly
	Comment          string
	UserID           uint
}

func (p *PaymentCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	err := validate.Struct(p)
	if err == nil {
		return errorMessages, true
	}
	for _, _err := range err.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Finance.Payment.%s", _err.Field()),
		})
		errorMessages[_err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", _err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}
	return errorMessages, len(errorMessages) == 0
}

func (p *PaymentCreateDTO) ToEntity() payment.Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		panic(err)
	}

	defaultCategory := paymentcategory.New("Uncategorized")

	return payment.New(
		p.Amount,
		defaultCategory,
		payment.WithCounterpartyID(p.CounterpartyID),
		payment.WithComment(p.Comment),
		payment.WithAccount(moneyaccount.New("", currency.Currency{}, moneyaccount.WithID(p.AccountID))),
		payment.WithUser(user.New(
			"",
			"",
			email,
			"",
			user.WithID(p.UserID),
		)),
		payment.WithTransactionDate(time.Time(p.TransactionDate)),
		payment.WithAccountingPeriod(time.Time(p.AccountingPeriod)),
	)
}

func (p *PaymentUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := validate.Struct(p)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Finance.Payment.%s", err.Field()),
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

func (p *PaymentUpdateDTO) ToEntity(id uint) payment.Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		panic(err)
	}

	defaultCategory := paymentcategory.New("Uncategorized")

	return payment.New(
		p.Amount,
		defaultCategory,
		payment.WithID(id),
		payment.WithTenantID(uuid.Nil),
		payment.WithCounterpartyID(p.CounterpartyID),
		payment.WithComment(p.Comment),
		payment.WithAccount(moneyaccount.New("", currency.Currency{}, moneyaccount.WithID(p.AccountID))),
		payment.WithUser(user.New(
			"",
			"",
			email,
			"",
			user.WithID(p.UserID),
		)),
		payment.WithTransactionDate(time.Time(p.TransactionDate)),
		payment.WithAccountingPeriod(time.Time(p.AccountingPeriod)),
		payment.WithCreatedAt(time.Now()),
		payment.WithUpdatedAt(time.Now()),
	)
}
