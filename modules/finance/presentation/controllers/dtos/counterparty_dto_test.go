package dtos

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func setupLocalizer(t *testing.T) context.Context {
	t.Helper()

	bundle := i18n.NewBundle(language.English)
	err := bundle.AddMessages(language.English,
		&i18n.Message{
			ID:    "Counterparties.Single.TIN",
			Other: "TIN",
		},
		&i18n.Message{
			ID:    "Counterparties.Single.Name",
			Other: "Name",
		},
		&i18n.Message{
			ID:    "ValidationErrors.invalidTIN",
			Other: "Invalid TIN format: {{.Details}}",
		},
		&i18n.Message{
			ID:    "ValidationErrors.required",
			Other: "This field is required",
		},
	)
	require.NoError(t, err)

	localizer := i18n.NewLocalizer(bundle, "en")
	return intl.WithLocalizer(context.Background(), localizer)
}

func TestCounterpartyCreateDTO_Ok_ValidTIN(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "123456789", // Valid Uzbekistan TIN (9 digits, numeric)
		Name:         "Test Company",
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.True(t, ok)
	assert.Empty(t, errors)
}

func TestCounterpartyCreateDTO_Ok_InvalidTIN_NonNumeric(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "12345678a", // Invalid: contains letter
		Name:         "Test Company",
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.False(t, ok)
	assert.Contains(t, errors, "TIN")
	assert.Contains(t, errors["TIN"], "TIN must contain only numbers")
}

func TestCounterpartyCreateDTO_Ok_InvalidTIN_WrongLength(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "12345678", // Invalid: 8 digits instead of 9
		Name:         "Test Company",
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.False(t, ok)
	assert.Contains(t, errors, "TIN")
	assert.Contains(t, errors["TIN"], "TIN must be exactly 9 digits")
}

func TestCounterpartyCreateDTO_Ok_EmptyTIN(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "", // Empty TIN should be allowed
		Name:         "Test Company",
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.True(t, ok)
	assert.Empty(t, errors)
}

func TestCounterpartyCreateDTO_Ok_RequiredFieldMissing(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "123456789",
		Name:         "", // Missing required field
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.False(t, ok)
	assert.Contains(t, errors, "Name")
	assert.Contains(t, errors["Name"], "This field is required")
}

func TestCounterpartyCreateDTO_Ok_MultipleErrors(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyCreateDTO{
		TIN:          "invalid", // Invalid TIN
		Name:         "",        // Missing required field
		Type:         "CUSTOMER",
		LegalType:    "LLC",
		LegalAddress: "Test Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.False(t, ok)
	assert.Contains(t, errors, "TIN")
	assert.Contains(t, errors, "Name")
	assert.Contains(t, errors["TIN"], "Invalid TIN format")
	assert.Contains(t, errors["Name"], "This field is required")
}

func TestCounterpartyUpdateDTO_Ok_ValidTIN(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyUpdateDTO{
		TIN:          "123456789", // Valid Uzbekistan TIN
		Name:         "Updated Company",
		Type:         "SUPPLIER",
		LegalType:    "JSC",
		LegalAddress: "Updated Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.True(t, ok)
	assert.Empty(t, errors)
}

func TestCounterpartyUpdateDTO_Ok_InvalidTIN(t *testing.T) {
	ctx := setupLocalizer(t)

	dto := &CounterpartyUpdateDTO{
		TIN:          "12345", // Invalid: too short
		Name:         "Updated Company",
		Type:         "SUPPLIER",
		LegalType:    "JSC",
		LegalAddress: "Updated Address",
	}

	errors, ok := dto.Ok(ctx)
	assert.False(t, ok)
	assert.Contains(t, errors, "TIN")
	assert.Contains(t, errors["TIN"], "TIN must be exactly 9 digits")
}
