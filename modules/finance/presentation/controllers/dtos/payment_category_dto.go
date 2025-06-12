package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type PaymentCategoryCreateDTO struct {
	Name        string `validate:"required"`
	Description string
}

type PaymentCategoryUpdateDTO struct {
	Name        string
	Description string
}

func (p *PaymentCategoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Finance.PaymentCategory.%s", err.Field()),
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

func (p *PaymentCategoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Finance.PaymentCategory.%s", err.Field()),
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

func (p *PaymentCategoryCreateDTO) ToEntity() (paymentcategory.PaymentCategory, error) {
	return paymentcategory.New(
		p.Name,
		paymentcategory.WithDescription(p.Description),
	), nil
}

func (p *PaymentCategoryUpdateDTO) ToEntity(id uuid.UUID) (paymentcategory.PaymentCategory, error) {
	return paymentcategory.New(
		p.Name,
		paymentcategory.WithID(id),
		paymentcategory.WithDescription(p.Description),
	), nil
}

func (p *PaymentCategoryUpdateDTO) Apply(existing paymentcategory.PaymentCategory) (paymentcategory.PaymentCategory, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	existing = existing.
		UpdateName(p.Name).
		UpdateDescription(p.Description)

	return existing, nil
}
