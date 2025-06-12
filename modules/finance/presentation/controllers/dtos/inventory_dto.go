package dtos

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type InventoryCreateDTO struct {
	Name         string `validate:"required"`
	Description  string
	CurrencyCode string
	Price        float64 `validate:"gte=0"`
	Quantity     int     `validate:"gte=0"`
}

type InventoryUpdateDTO struct {
	Name         string `validate:"required"`
	Description  string
	CurrencyCode string
	Price        float64 `validate:"gte=0"`
	Quantity     int     `validate:"gte=0"`
}

func (dto *InventoryCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Inventory.Single.%s", err.Field()),
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

func (dto *InventoryCreateDTO) ToEntity() (inventory.Inventory, error) {
	opts := []inventory.Option{}

	if dto.Description != "" {
		opts = append(opts, inventory.WithDescription(dto.Description))
	}

	// Create Money object from price and currency
	currencyCode := "USD" // Default currency
	if dto.CurrencyCode != "" {
		currencyCode = dto.CurrencyCode
	}
	price := money.NewFromFloat(dto.Price, currencyCode)

	return inventory.New(
		dto.Name,
		price,
		dto.Quantity,
		opts...,
	), nil
}

func (dto *InventoryUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		panic(intl.ErrNoLocalizer)
	}
	errorMessages := map[string]string{}
	errs := constants.Validate.Struct(dto)
	if errs == nil {
		return errorMessages, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		translatedFieldName := l.MustLocalize(&i18n.LocalizeConfig{
			MessageID: fmt.Sprintf("Inventory.Single.%s", err.Field()),
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

func (dto *InventoryUpdateDTO) ToEntity(id uuid.UUID) (inventory.Inventory, error) {
	opts := []inventory.Option{
		inventory.WithID(id),
	}

	if dto.Description != "" {
		opts = append(opts, inventory.WithDescription(dto.Description))
	}

	// Create Money object from price and currency
	currencyCode := "USD" // Default currency
	if dto.CurrencyCode != "" {
		currencyCode = dto.CurrencyCode
	}
	price := money.NewFromFloat(dto.Price, currencyCode)

	return inventory.New(
		dto.Name,
		price,
		dto.Quantity,
		opts...,
	), nil
}

func (dto *InventoryUpdateDTO) Apply(existing inventory.Inventory) (inventory.Inventory, error) {
	if existing.ID() == uuid.Nil {
		return nil, errors.New("id cannot be nil")
	}

	// Create Money object from price and currency if provided
	var price *money.Money
	if dto.CurrencyCode != "" {
		price = money.NewFromFloat(dto.Price, dto.CurrencyCode)
	} else {
		price = existing.Price()
	}

	existing = existing.
		UpdateName(dto.Name).
		UpdateDescription(dto.Description).
		UpdatePrice(price).
		UpdateQuantity(dto.Quantity)

	return existing, nil
}
