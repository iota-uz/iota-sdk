package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type CreateOrderDTO struct {
	PositionIDs []uint        `validate:"required"`
	Quantity    map[uint]uint `validate:"required"`
}

type UpdateOrderDTO struct {
	PositionIDs []uint
	Quantities  map[uint]uint
}

func (d *CreateOrderDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
		if err.Tag() == "required" && err.Field() == "PositionIDs" {
			translatedField := l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "WarehouseOrders.Single.PositionIDs",
			})
			errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
				MessageID: "ValidationErrors.emptySelect",
				TemplateData: map[string]string{
					"Field": translatedField,
				},
			})
			continue
		}
		translatedField := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("WarehouseOrders.Single.%s", err.Field()),
		})

		errorMessages[err.Field()] = l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("ValidationErrors.%s", err.Tag()),
			TemplateData: map[string]string{
				"Field": translatedField,
			},
		})
	}
	return errorMessages, len(errorMessages) == 0
}

func (d *UpdateOrderDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("WarehouseOrders.Single.%s", err.Field()),
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
