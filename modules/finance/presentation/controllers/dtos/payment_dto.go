package dtos

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

var paymentFieldTranslations = map[string]string{
	"CounterpartyID":    "Payments.Single.CounterpartyID.Label",
	"AccountID":         "Payments.Single.Account",
	"Amount":            "Payments.Single.Amount",
	"TransactionDate":   "Payments.Single.Date",
	"AccountingPeriod":  "Payments.Single.AccountingPeriod",
	"Comment":           "Payments.Single.Comment",
	"UserID":            "Payments.Single.User",
	"PaymentCategoryID": "Payments.Single.Category",
}

func validatePaymentDTO(ctx context.Context, data interface{}) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}

	errorMessages := map[string]string{}
	err := validate.Struct(data)
	if err == nil {
		return errorMessages, true
	}

	for _, validationErr := range err.(validator.ValidationErrors) {
		fieldName := validationErr.Field()
		translationKey, exists := paymentFieldTranslations[fieldName]
		if !exists {
			translationKey = fieldName
		}

		var translatedFieldName string
		if exists {
			translatedFieldName = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: translationKey,
			})
		} else {
			translatedFieldName = fieldName
		}

		errorMessages[fieldName] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", validationErr.Tag()),
			TemplateData: map[string]string{
				"Field": translatedFieldName,
			},
		})
	}

	return errorMessages, len(errorMessages) == 0
}

type PaymentCreateDTO struct {
	Amount            float64         `validate:"required,gt=0"`
	AccountID         string          `validate:"required,uuid"`
	TransactionDate   shared.DateOnly `validate:"required"`
	AccountingPeriod  shared.DateOnly `validate:"required"`
	CounterpartyID    string          `validate:"required,uuid"`
	PaymentCategoryID string          `validate:"required,uuid"`
	UserID            uint            `validate:"required"`
	Comment           string
}

type PaymentUpdateDTO struct {
	Amount            float64 `validate:"gt=0"`
	AccountID         string  `validate:"omitempty,uuid"`
	CounterpartyID    string  `validate:"omitempty,uuid"`
	PaymentCategoryID string  `validate:"omitempty,uuid"`
	TransactionDate   shared.DateOnly
	AccountingPeriod  shared.DateOnly
	Comment           string
	UserID            uint
}

func (p *PaymentCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return validatePaymentDTO(ctx, p)
}

func (p *PaymentCreateDTO) ToEntity(tenantID uuid.UUID, category paymentcategory.PaymentCategory) payment.Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		panic(err)
	}

	accountID, err := uuid.Parse(p.AccountID)
	if err != nil {
		panic(err)
	}

	counterpartyID, err := uuid.Parse(p.CounterpartyID)
	if err != nil {
		panic(err)
	}

	// Create Money object from amount
	amount := money.NewFromFloat(p.Amount, "USD")

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")

	return payment.New(
		amount,
		category,
		payment.WithTenantID(tenantID),
		payment.WithCounterpartyID(counterpartyID),
		payment.WithComment(p.Comment),
		payment.WithAccount(moneyaccount.New("", defaultBalance, moneyaccount.WithID(accountID))),
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
	return validatePaymentDTO(ctx, p)
}

func (p *PaymentUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID, category paymentcategory.PaymentCategory) payment.Payment {
	email, err := internet.NewEmail("payment@system.internal")
	if err != nil {
		panic(err)
	}

	var accountID uuid.UUID
	if p.AccountID != "" {
		accountID, err = uuid.Parse(p.AccountID)
		if err != nil {
			panic(err)
		}
	}

	var counterpartyID uuid.UUID
	if p.CounterpartyID != "" {
		counterpartyID, err = uuid.Parse(p.CounterpartyID)
		if err != nil {
			panic(err)
		}
	}

	// Create Money object from amount
	amount := money.NewFromFloat(p.Amount, "USD")

	// Create default money account with zero balance
	defaultBalance := money.New(0, "USD")

	return payment.New(
		amount,
		category,
		payment.WithID(id),
		payment.WithTenantID(tenantID),
		payment.WithCounterpartyID(counterpartyID),
		payment.WithComment(p.Comment),
		payment.WithAccount(moneyaccount.New("", defaultBalance, moneyaccount.WithID(accountID))),
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

func (p *PaymentUpdateDTO) Apply(existing payment.Payment, category paymentcategory.PaymentCategory, u user.User) (payment.Payment, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	var counterpartyID uuid.UUID
	var err error
	if p.CounterpartyID != "" {
		counterpartyID, err = uuid.Parse(p.CounterpartyID)
		if err != nil {
			return nil, fmt.Errorf("invalid counterparty ID: %w", err)
		}
	}

	// Create Money object from amount if provided
	var amount *money.Money
	if p.Amount > 0 {
		amount = money.NewFromFloat(p.Amount, "USD")
	} else {
		amount = existing.Amount()
	}

	existing = existing.
		UpdateAmount(amount).
		UpdateCategory(category).
		UpdateCounterpartyID(counterpartyID).
		UpdateComment(p.Comment).
		UpdateTransactionDate(time.Time(p.TransactionDate)).
		UpdateAccountingPeriod(time.Time(p.AccountingPeriod))

	return existing, nil
}
