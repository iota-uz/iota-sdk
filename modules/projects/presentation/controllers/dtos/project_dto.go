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
)

// ProjectCreateDTO gives the project a contract only when both its amount and
// currency are given.
type ProjectCreateDTO struct {
	CounterpartyID   string  `validate:"required,uuid"`
	Name             string  `validate:"required,min=2,max=255"`
	Description      string  `validate:"max=1000"`
	ContractAmount   float64 `validate:"gte=0"`
	ContractCurrency string  `validate:"omitempty,len=3"`
}

type ProjectUpdateDTO struct {
	CounterpartyID   string  `validate:"required,uuid"`
	Name             string  `validate:"required,min=2,max=255"`
	Description      string  `validate:"max=1000"`
	ContractAmount   float64 `validate:"gte=0"`
	ContractCurrency string  `validate:"omitempty,len=3"`
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

// contractErrors asks for the currency once a contract amount is given.
func contractErrors(l *i18n.Localizer, amount float64, currency string, errorMessages map[string]string) {
	if amount > 0 && currency == "" {
		errorMessages["ContractCurrency"] = fieldError(l, "Projects.Single", "ContractCurrency", "required")
	}
}

func contract(amount float64, currency string) *money.Money {
	if amount == 0 || currency == "" {
		return nil
	}
	return money.NewFromFloat(amount, currency)
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
