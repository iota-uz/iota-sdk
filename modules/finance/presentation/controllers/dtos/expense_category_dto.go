package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type ExpenseCategoryCreateDTO struct {
	Name        string `validate:"required"`
	Description string
}

type ExpenseCategoryUpdateDTO struct {
	Name        string `validate:"required"`
	Description string
}

func (e *ExpenseCategoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ExpenseCategories.Single.%s", err.Field()),
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

func (e *ExpenseCategoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(e)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ExpenseCategories.Single.%s", err.Field()),
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

func (e *ExpenseCategoryCreateDTO) ToEntity(tenantID uuid.UUID) (category.ExpenseCategory, error) {
	return category.New(
		e.Name,
		category.WithTenantID(tenantID),
		category.WithDescription(e.Description),
	), nil
}

func (e *ExpenseCategoryUpdateDTO) ToEntity(id uuid.UUID, tenantID uuid.UUID) (category.ExpenseCategory, error) {
	return category.New(
		e.Name,
		category.WithID(id),
		category.WithTenantID(tenantID),
		category.WithDescription(e.Description),
	), nil
}

func (e *ExpenseCategoryUpdateDTO) Apply(existing category.ExpenseCategory) (category.ExpenseCategory, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	existing = existing.
		UpdateName(e.Name).
		UpdateDescription(e.Description)

	return existing, nil
}
