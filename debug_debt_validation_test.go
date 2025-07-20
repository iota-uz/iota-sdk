package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/require"
)

func TestDebugDebtValidation(t *testing.T) {
	adminUser := itf.User()
	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()

	// Create currencies first
	err := env.App.Service(services.CurrencyService{}).(*services.CurrencyService).Create(env.Ctx, &currency.USD)
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

	// Test the DTO validation directly
	dto := &dtos.DebtCreateDTO{
		CounterpartyID: createdCounterparty.ID().String(),
		Amount:         500.75,
		Type:           "RECEIVABLE",
		Description:    "New test debt",
		DueDate:        shared.DateOnly(now),
	}

	fmt.Printf("=== DTO VALUES ===\n")
	fmt.Printf("CounterpartyID: %s\n", dto.CounterpartyID)
	fmt.Printf("Amount: %f\n", dto.Amount)
	fmt.Printf("Type: %s\n", dto.Type)
	fmt.Printf("Description: %s\n", dto.Description)
	fmt.Printf("DueDate: %s\n", time.Time(dto.DueDate).Format(time.DateOnly))
	fmt.Printf("==================\n")

	// Validate the DTO
	errors, isValid := dto.Ok(env.Ctx)

	fmt.Printf("=== VALIDATION RESULT ===\n")
	fmt.Printf("IsValid: %t\n", isValid)
	fmt.Printf("Errors: %+v\n", errors)
	fmt.Printf("========================\n")

	if !isValid {
		for field, message := range errors {
			fmt.Printf("Field '%s': %s\n", field, message)
		}
	}

	// Also test with empty/invalid values
	fmt.Printf("\n=== TESTING INVALID DTO ===\n")
	invalidDTO := &dtos.DebtCreateDTO{
		CounterpartyID: "",
		Amount:         0,
		Type:           "",
		Description:    "",
		DueDate:        shared.DateOnly{},
	}

	invalidErrors, invalidIsValid := invalidDTO.Ok(env.Ctx)
	fmt.Printf("IsValid: %t\n", invalidIsValid)
	fmt.Printf("Errors: %+v\n", invalidErrors)

	if !invalidIsValid {
		for field, message := range invalidErrors {
			fmt.Printf("Field '%s': %s\n", field, message)
		}
	}
}
