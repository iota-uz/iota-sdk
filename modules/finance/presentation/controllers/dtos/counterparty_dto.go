package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
)

type CounterpartyCreateDTO struct {
	TIN          string
	Name         string `validate:"required,min=2,max=255"`
	Type         string `validate:"required"`
	LegalType    string `validate:"required"`
	LegalAddress string `validate:"max=500"`
}

type CounterpartyUpdateDTO struct {
	TIN          string
	Name         string `validate:"required,min=2,max=255"`
	Type         string `validate:"required"`
	LegalType    string `validate:"required"`
	LegalAddress string `validate:"max=500"`
}

func (dto *CounterpartyCreateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Counterparties.Single.%s", err.Field()),
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

func (dto *CounterpartyCreateDTO) ToEntity(tenantID uuid.UUID) (counterparty.Counterparty, error) {
	var tin tax.Tin
	var err error
	if dto.TIN != "" {
		tin, err = tax.NewTin(dto.TIN, country.Uzbekistan)
		if err != nil {
			return nil, fmt.Errorf("invalid TIN: %w", err)
		}
	}

	cType, err := counterparty.ParseType(dto.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid type: %w", err)
	}

	legalType, err := counterparty.ParseLegalType(dto.LegalType)
	if err != nil {
		return nil, fmt.Errorf("invalid legal type: %w", err)
	}

	return counterparty.New(
		dto.Name,
		cType,
		legalType,
		counterparty.WithTenantID(tenantID),
		counterparty.WithTin(tin),
		counterparty.WithLegalAddress(dto.LegalAddress),
	), nil
}

func (dto *CounterpartyUpdateDTO) Ok(ctx context.Context) (map[string]string, bool) {
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
			MessageID: fmt.Sprintf("Counterparties.Single.%s", err.Field()),
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

func (dto *CounterpartyUpdateDTO) Apply(existing counterparty.Counterparty) (counterparty.Counterparty, error) {
	var tin tax.Tin
	var err error
	if dto.TIN != "" {
		tin, err = tax.NewTin(dto.TIN, country.Uzbekistan)
		if err != nil {
			return nil, fmt.Errorf("invalid TIN: %w", err)
		}
	}

	cType, err := counterparty.ParseType(dto.Type)
	if err != nil {
		return nil, fmt.Errorf("invalid type: %w", err)
	}

	legalType, err := counterparty.ParseLegalType(dto.LegalType)
	if err != nil {
		return nil, fmt.Errorf("invalid legal type: %w", err)
	}

	existing.SetTin(tin)
	existing.SetName(dto.Name)
	existing.SetType(cType)
	existing.SetLegalType(legalType)
	existing.SetLegalAddress(dto.LegalAddress)

	return existing, nil
}
