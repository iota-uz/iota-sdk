package main

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreServices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func setupLocalizer(t *testing.T) context.Context {
	t.Helper()

	bundle := i18n.NewBundle(language.English)
	err := bundle.AddMessages(language.English,
		&i18n.Message{
			ID:    "Debts.Single.CounterpartyID",
			Other: "Counterparty",
		},
		&i18n.Message{
			ID:    "Debts.Single.Amount",
			Other: "Amount",
		},
		&i18n.Message{
			ID:    "Debts.Single.Type",
			Other: "Type",
		},
		&i18n.Message{
			ID:    "Debts.Single.Description",
			Other: "Description",
		},
		&i18n.Message{
			ID:    "Debts.Single.DueDate",
			Other: "Due Date",
		},
		&i18n.Message{
			ID:    "ValidationErrors.required",
			Other: "{{.Field}} is required",
		},
		&i18n.Message{
			ID:    "ValidationErrors.min",
			Other: "{{.Field}} must be greater than 0",
		},
		&i18n.Message{
			ID:    "ValidationErrors.oneof",
			Other: "{{.Field}} must be one of the allowed values",
		},
	)
	require.NoError(t, err)

	localizer := i18n.NewLocalizer(bundle, "en")
	return intl.WithLocalizer(context.Background(), localizer)
}

func TestDebugDebtValidation(t *testing.T) {
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	// Create currencies first
	currencyDTO := &currency.CreateDTO{
		Code:   string(currency.USD.Code),
		Name:   currency.USD.Name,
		Symbol: string(currency.USD.Symbol),
	}
	err := env.App.Service(coreServices.CurrencyService{}).(*coreServices.CurrencyService).Create(env.Ctx, currencyDTO)
	require.NoError(t, err)

	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Debug Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	now := time.Now()

	// Set up a context with localizer for DTO validation
	ctx := setupLocalizer(t)

	// Test the DTO validation directly
	dto := &dtos.DebtCreateDTO{
		CounterpartyID: createdCounterparty.ID().String(),
		Amount:         500.75,
		Type:           "RECEIVABLE",
		Description:    "New test debt",
		DueDate:        shared.DateOnly(now),
	}

	// Validate the DTO
	errors, isValid := dto.Ok(ctx)

	if !isValid {
		for field, message := range errors {
			t.Logf("Field '%s': %s\n", field, message)
		}
	}

	// Also test with empty/invalid values
	invalidDTO := &dtos.DebtCreateDTO{
		CounterpartyID: "",
		Amount:         0,
		Type:           "",
		Description:    "",
		DueDate:        shared.DateOnly{},
	}

	invalidErrors, invalidIsValid := invalidDTO.Ok(ctx)

	if !invalidIsValid {
		for field, message := range invalidErrors {
			t.Logf("Field '%s': %s\n", field, message)
		}
	}
}
