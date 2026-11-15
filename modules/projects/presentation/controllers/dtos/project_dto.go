// Package dtos provides this package.
package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

// ProjectCreateDTO gives the project a contract only when both its amount and
// currency are given.
type ProjectCreateDTO struct {
	CounterpartyID   string `validate:"required,uuid"`
	Name             string `validate:"required,min=2,max=255"`
	Description      string `validate:"max=1000"`
	ContractAmount   shared.Amount
	ContractCurrency string `validate:"omitempty,len=3"`
}

type ProjectUpdateDTO struct {
	CounterpartyID   string `validate:"required,uuid"`
	Name             string `validate:"required,min=2,max=255"`
	Description      string `validate:"max=1000"`
	ContractAmount   shared.Amount
	ContractCurrency string `validate:"omitempty,len=3"`
}

func validationErrors(l *i18n.Localizer, fields string, errs error) map[string]string {
	errorMessages := map[string]string{}
	if errs == nil {
		return errorMessages
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errorMessages[err.Field()] = fieldError(l, fields, err.Field(), err.Tag())
	}
	return errorMessages
}

// fieldError translates a validation error of a field whose label is under
// the fields key.
func fieldError(l *i18n.Localizer, fields, field, tag string) string {
	return l.MustLocalize(&i18n.LocalizeConfig{
		MessageID: fmt.Sprintf("ValidationErrors.%s", tag),
		TemplateData: map[string]string{
			"Field": l.MustLocalize(&i18n.LocalizeConfig{MessageID: fmt.Sprintf("%s.%s", fields, field)}),
		},
	})
}

// amountErrors reports an amount that is not a number or is below zero.
func amountErrors(l *i18n.Localizer, fields, field string, amount shared.Amount, errorMessages map[string]string) {
	switch {
	case amount.Err() != nil:
		errorMessages[field] = l.MustLocalize(&i18n.LocalizeConfig{MessageID: "Projects.Errors.InvalidAmount"})
	case amount.IsNegative():
		errorMessages[field] = fieldError(l, fields, field, "gte")
	}
}

// contractErrors checks the contract amount and asks for the currency once
// an amount is given.
func contractErrors(l *i18n.Localizer, amount shared.Amount, currency string, errorMessages map[string]string) {
	amountErrors(l, "Projects.Single", "ContractAmount", amount, errorMessages)
	if amount.IsPositive() && currency == "" {
		errorMessages["ContractCurrency"] = fieldError(l, "Projects.Single", "ContractCurrency", "required")
	}
}

func contract(amount shared.Amount, currency string) *money.Money {
	if !amount.IsPositive() || currency == "" {
		return nil
	}
	return money.NewFromFloat(amount.Float64(), currency)
}

func (dto *ProjectCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := validationErrors(l, "Projects.Single", constants.Validate.Struct(dto))
	contractErrors(l, dto.ContractAmount, dto.ContractCurrency, errorMessages)
	return errorMessages, len(errorMessages) == 0
}

func (dto *ProjectCreateDTO) ToEntity(tenantID uuid.UUID) (project.Project, error) {
	counterpartyID, err := uuid.Parse(dto.CounterpartyID)
	if err != nil {
		return nil, err
	}

	entity := project.New(
		dto.Name,
		counterpartyID,
		project.WithTenantID(tenantID),
		project.WithDescription(dto.Description),
		project.WithContract(contract(dto.ContractAmount, dto.ContractCurrency)),
	)
	return entity, nil
}

func (dto *ProjectUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := validationErrors(l, "Projects.Single", constants.Validate.Struct(dto))
	contractErrors(l, dto.ContractAmount, dto.ContractCurrency, errorMessages)
	return errorMessages, len(errorMessages) == 0
}

func (dto *ProjectUpdateDTO) Apply(existing project.Project) (project.Project, error) {
	counterpartyID, err := uuid.Parse(dto.CounterpartyID)
	if err != nil {
		return nil, err
	}

	updated := existing.UpdateCounterpartyID(counterpartyID)
	updated = updated.UpdateName(dto.Name)
	updated = updated.UpdateDescription(dto.Description)
	updated = updated.UpdateContract(contract(dto.ContractAmount, dto.ContractCurrency))
	return updated, nil
}
