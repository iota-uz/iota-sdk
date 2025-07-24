package dtos

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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
	validationErrors := make(serrors.ValidationErrors)

	// Process standard validator errors
	errs := constants.Validate.Struct(dto)
	if errs != nil {
		getFieldLocaleKey := func(field string) string {
			return fmt.Sprintf("Counterparties.Single.%s", field)
		}

		// Process validator errors to our custom format
		for field, err := range serrors.ProcessValidatorErrors(errs.(validator.ValidationErrors), getFieldLocaleKey) {
			validationErrors[field] = err
		}
	}

	// Custom TIN validation
	if dto.TIN != "" {
		if err := tax.ValidateTin(dto.TIN, country.Uzbekistan); err != nil {
			validationErrors["TIN"] = serrors.NewInvalidTINError(
				"TIN",
				"Counterparties.Single.TIN",
				err.Error(),
			)
		}
	}

	// Localize all validation errors
	errorMessages := serrors.LocalizeValidationErrors(validationErrors, l)
	return errorMessages, len(errorMessages) == 0
}

func (dto *CounterpartyCreateDTO) ToEntity(tenantID uuid.UUID) (counterparty.Counterparty, error) {
	var tin tax.Tin
	if dto.TIN != "" {
		// TIN validation is already done in Ok() method, so we can safely create it here
		tin, _ = tax.NewTin(dto.TIN, country.Uzbekistan)
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
	validationErrors := make(serrors.ValidationErrors)

	// Process standard validator errors
	errs := constants.Validate.Struct(dto)
	if errs != nil {
		getFieldLocaleKey := func(field string) string {
			return fmt.Sprintf("Counterparties.Single.%s", field)
		}

		// Process validator errors to our custom format
		for field, err := range serrors.ProcessValidatorErrors(errs.(validator.ValidationErrors), getFieldLocaleKey) {
			validationErrors[field] = err
		}
	}

	// Custom TIN validation
	if dto.TIN != "" {
		if err := tax.ValidateTin(dto.TIN, country.Uzbekistan); err != nil {
			validationErrors["TIN"] = serrors.NewInvalidTINError(
				"TIN",
				"Counterparties.Single.TIN",
				err.Error(),
			)
		}
	}

	// Localize all validation errors
	errorMessages := serrors.LocalizeValidationErrors(validationErrors, l)
	return errorMessages, len(errorMessages) == 0
}

func (dto *CounterpartyUpdateDTO) Apply(existing counterparty.Counterparty) (counterparty.Counterparty, error) {
	var tin tax.Tin
	if dto.TIN != "" {
		// TIN validation is already done in Ok() method, so we can safely create it here
		tin, _ = tax.NewTin(dto.TIN, country.Uzbekistan)
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
