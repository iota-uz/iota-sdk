package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type ExpenseCategoryCreateDTO struct {
	Name         string  `validate:"required"`
	Amount       float64 `validate:"required,gt=0"`
	CurrencyCode string  `validate:"required,len=3"`
	Description  string
}

type ExpenseCategoryUpdateDTO struct {
	Name         string
	Amount       float64 `validate:"gt=0"`
	CurrencyCode string  `validate:"len=3"`
	Description  string
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
			MessageID: fmt.Sprintf("Finance.ExpenseCategory.%s", err.Field()),
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
			MessageID: fmt.Sprintf("Finance.ExpenseCategory.%s", err.Field()),
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

func (e *ExpenseCategoryCreateDTO) ToEntity() (category.ExpenseCategory, error) {
	code, err := currency.NewCode(e.CurrencyCode)
	if err != nil {
		return nil, err
	}

	return category.New(
		e.Name,
		e.Amount,
		&currency.Currency{Code: code},
		category.WithDescription(e.Description),
	), nil
}

func (e *ExpenseCategoryUpdateDTO) ToEntity(id uint) (category.ExpenseCategory, error) {
	code, err := currency.NewCode(e.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return category.New(
		e.Name,
		e.Amount,
		&currency.Currency{Code: code},
		category.WithID(id),
		category.WithDescription(e.Description),
	), nil
}
