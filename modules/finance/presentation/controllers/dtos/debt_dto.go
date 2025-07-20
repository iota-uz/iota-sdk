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
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

var debtFieldTranslations = map[string]string{
	"CounterpartyID": "Debts.Single.CounterpartyID",
	"Amount":         "Debts.Single.Amount",
	"Type":           "Debts.Single.Type",
	"Description":    "Debts.Single.Description",
	"DueDate":        "Debts.Single.DueDate",
}

func validateDebtDTO(ctx context.Context, data interface{}) (map[string]string, bool) {
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
		translationKey, exists := debtFieldTranslations[fieldName]
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

type DebtCreateDTO struct {
	Amount         float64 `validate:"required,gt=0"`
	CounterpartyID string  `validate:"required,uuid"`
	Type           string  `validate:"required,oneof=RECEIVABLE PAYABLE"`
	Description    string  `validate:"required"`
	DueDate        shared.DateOnly
}

type DebtUpdateDTO struct {
	Amount         float64 `validate:"gt=0"`
	CounterpartyID string  `validate:"omitempty,uuid"`
	Type           string  `validate:"omitempty,oneof=RECEIVABLE PAYABLE"`
	Description    string
	DueDate        shared.DateOnly
	Status         string `validate:"omitempty,oneof=PENDING SETTLED PARTIAL WRITTEN_OFF"`
}

type DebtSettleDTO struct {
	SettlementAmount        float64 `validate:"required,gt=0"`
	SettlementTransactionID string  `validate:"omitempty,uuid"`
}

func (d *DebtCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return validateDebtDTO(ctx, d)
}

func (d *DebtCreateDTO) ToEntity(tenantID uuid.UUID) debt.Debt {
	counterpartyID, err := uuid.Parse(d.CounterpartyID)
	if err != nil {
		panic(err)
	}

	debtType := debt.DebtType(d.Type)
	amount := money.NewFromFloat(d.Amount, "USD")

	email, err := internet.NewEmail("debt@system.internal")
	if err != nil {
		panic(err)
	}

	opts := []debt.Option{
		debt.WithTenantID(tenantID),
		debt.WithCounterpartyID(counterpartyID),
		debt.WithDescription(d.Description),
		debt.WithUser(user.New("", "", email, "")),
	}

	if !time.Time(d.DueDate).IsZero() {
		dueDate := time.Time(d.DueDate)
		opts = append(opts, debt.WithDueDate(&dueDate))
	}

	return debt.New(debtType, amount, opts...)
}

func (d *DebtUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return validateDebtDTO(ctx, d)
}

func (d *DebtUpdateDTO) Apply(existing debt.Debt) (debt.Debt, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	updated := existing

	if d.Amount > 0 {
		amount := money.NewFromFloat(d.Amount, "USD")
		updated = updated.UpdateOriginalAmount(amount).UpdateOutstandingAmount(amount)
	}

	if d.CounterpartyID != "" {
		counterpartyID, err := uuid.Parse(d.CounterpartyID)
		if err != nil {
			return nil, fmt.Errorf("invalid counterparty ID: %w", err)
		}
		updated = updated.UpdateCounterpartyID(counterpartyID)
	}

	if d.Type != "" {
		debtType := debt.DebtType(d.Type)
		updated = updated.UpdateType(debtType)
	}

	if d.Description != "" {
		updated = updated.UpdateDescription(d.Description)
	}

	if d.Status != "" {
		status := debt.DebtStatus(d.Status)
		updated = updated.UpdateStatus(status)
	}

	if !time.Time(d.DueDate).IsZero() {
		dueDate := time.Time(d.DueDate)
		updated = updated.UpdateDueDate(&dueDate)
	}

	return updated, nil
}

func (d *DebtSettleDTO) Ok(ctx context.Context) (map[string]string, bool) {
	return validateDebtDTO(ctx, d)
}

func (d *DebtSettleDTO) GetTransactionID() *uuid.UUID {
	if d.SettlementTransactionID == "" {
		return nil
	}
	transactionID, err := uuid.Parse(d.SettlementTransactionID)
	if err != nil {
		panic(err)
	}
	return &transactionID
}
