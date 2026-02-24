package services

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/testkit/domain/schemas"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type TestDataService struct {
	app             application.Application
	resetService    *ResetService
	populateService *PopulateService
}

func NewTestDataService(app application.Application) *TestDataService {
	return &TestDataService{
		app:             app,
		resetService:    NewResetService(app),
		populateService: NewPopulateService(app),
	}
}

func (s *TestDataService) ResetDatabase(ctx context.Context, reseedMinimal bool) error {
	logger := composables.UseLogger(ctx)

	// Perform database reset
	if err := s.resetService.TruncateAllTables(ctx); err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}

	logger.Info("Database tables truncated")

	// Reseed with minimal data if requested
	if reseedMinimal {
		if err := s.SeedScenario(ctx, "minimal"); err != nil {
			return fmt.Errorf("failed to reseed minimal data: %w", err)
		}
		logger.Info("Minimal data reseeded")
	}

	return nil
}

func (s *TestDataService) PopulateData(ctx context.Context, req *schemas.PopulateRequest) (map[string]interface{}, error) {
	return s.populateService.Execute(ctx, req)
}

func (s *TestDataService) SeedScenario(ctx context.Context, scenarioName string) error {
	scenario, exists := s.getScenario(scenarioName)
	if !exists {
		return fmt.Errorf("scenario '%s' not found", scenarioName)
	}

	_, err := s.PopulateData(ctx, scenario)
	return err
}

func (s *TestDataService) GetAvailableScenarios() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"name":        "minimal",
			"description": "Basic setup with default tenant and test user",
		},
		{
			"name":        "finance",
			"description": "Finance module focused setup with accounts, categories, and sample transactions",
		},
		{
			"name":        "warehouse",
			"description": "Warehouse module setup with units, products, and inventory",
		},
		{
			"name":        "comprehensive",
			"description": "Full setup with data across all modules",
		},
	}
}

func (s *TestDataService) getScenario(name string) (*schemas.PopulateRequest, bool) {
	scenarios := map[string]*schemas.PopulateRequest{
		"minimal": {
			Version: "1.0",
			Tenant: &schemas.TenantSpec{
				ID:     "00000000-0000-0000-0000-000000000001",
				Name:   "Default Test Tenant",
				Domain: "test.localhost",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "test@gmail.com",
						Password:  "TestPass123!",
						FirstName: "Test",
						LastName:  "User",
						Phone:     "+998901230001",
						Language:  "en",
						Ref:       "testUser",
					},
				},
			},
			Options: &schemas.OptionsSpec{
				ClearExisting:      false,
				ReturnIds:          true,
				ValidateReferences: true,
				StopOnError:        true,
			},
		},
		"finance": {
			Version: "1.0",
			Tenant: &schemas.TenantSpec{
				ID:     "00000000-0000-0000-0000-000000000001",
				Name:   "Default Test Tenant",
				Domain: "test.localhost",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "test@gmail.com",
						Password:  "TestPass123!",
						FirstName: "Test",
						LastName:  "User",
						Phone:     "+998901230001",
						Language:  "en",
						Ref:       "testUser",
					},
				},
				Finance: &schemas.FinanceSpec{
					MoneyAccounts: []schemas.MoneyAccountSpec{
						{
							Name:     "Main Cash Account",
							Currency: "USD",
							Balance:  1000.00,
							Type:     "cash",
							Ref:      "mainCash",
						},
						{
							Name:     "Bank Account",
							Currency: "USD",
							Balance:  5000.00,
							Type:     "bank",
							Ref:      "bankAccount",
						},
					},
					PaymentCategories: []schemas.PaymentCategorySpec{
						{
							Name: "Sales Revenue",
							Type: "income",
							Ref:  "salesRevenue",
						},
						{
							Name: "Service Income",
							Type: "income",
							Ref:  "serviceIncome",
						},
					},
					ExpenseCategories: []schemas.ExpenseCategorySpec{
						{
							Name: "Office Supplies",
							Type: "expense",
							Ref:  "officeSupplies",
						},
						{
							Name: "Utilities",
							Type: "expense",
							Ref:  "utilities",
						},
					},
					Payments: []schemas.PaymentSpec{
						{
							Amount:      1500.00,
							Date:        "2024-01-15",
							AccountRef:  "@moneyAccounts.mainCash",
							CategoryRef: "@paymentCategories.salesRevenue",
							Comment:     "January sales payment",
						},
					},
					Expenses: []schemas.ExpenseSpec{
						{
							Amount:      200.00,
							Date:        "2024-01-10",
							AccountRef:  "@moneyAccounts.bankAccount",
							CategoryRef: "@expenseCategories.officeSupplies",
							Comment:     "Office supplies purchase",
						},
					},
				},
			},
			Options: &schemas.OptionsSpec{
				ClearExisting:      false,
				ReturnIds:          true,
				ValidateReferences: true,
				StopOnError:        true,
			},
		},
		"warehouse": {
			Version: "1.0",
			Tenant: &schemas.TenantSpec{
				ID:     "00000000-0000-0000-0000-000000000001",
				Name:   "Default Test Tenant",
				Domain: "test.localhost",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "test@gmail.com",
						Password:  "TestPass123!",
						FirstName: "Test",
						LastName:  "User",
						Phone:     "+998901230001",
						Language:  "en",
						Ref:       "testUser",
					},
				},
				Warehouse: &schemas.WarehouseSpec{
					Units: []schemas.UnitSpec{
						{
							Title:      "Pieces",
							ShortTitle: "pcs",
							Ref:        "pieces",
						},
						{
							Title:      "Kilograms",
							ShortTitle: "kg",
							Ref:        "kilograms",
						},
					},
					Products: []schemas.ProductSpec{
						{
							Name:    "Widget A",
							UnitRef: "@units.pieces",
							Price:   25.50,
							Ref:     "widgetA",
						},
						{
							Name:    "Component B",
							UnitRef: "@units.kilograms",
							Price:   15.75,
							Ref:     "componentB",
						},
					},
				},
			},
			Options: &schemas.OptionsSpec{
				ClearExisting:      false,
				ReturnIds:          true,
				ValidateReferences: true,
				StopOnError:        true,
			},
		},
		"comprehensive": {
			Version: "1.0",
			Tenant: &schemas.TenantSpec{
				ID:     "00000000-0000-0000-0000-000000000001",
				Name:   "Default Test Tenant",
				Domain: "test.localhost",
			},
			Data: &schemas.DataSpec{
				Users: []schemas.UserSpec{
					{
						Email:     "test@gmail.com",
						Password:  "TestPass123!",
						FirstName: "Test",
						LastName:  "User",
						Phone:     "+998901230001",
						Language:  "en",
						Ref:       "testUser",
					},
					{
						Email:     "manager@test.com",
						Password:  "ManagerPass123!",
						FirstName: "Manager",
						LastName:  "User",
						Language:  "en",
						Ref:       "managerUser",
					},
				},
				Finance: &schemas.FinanceSpec{
					MoneyAccounts: []schemas.MoneyAccountSpec{
						{
							Name:     "Main Cash Account",
							Currency: "USD",
							Balance:  1000.00,
							Type:     "cash",
							Ref:      "mainCash",
						},
					},
					PaymentCategories: []schemas.PaymentCategorySpec{
						{
							Name: "Sales Revenue",
							Type: "income",
							Ref:  "salesRevenue",
						},
					},
					Payments: []schemas.PaymentSpec{
						{
							Amount:      1500.00,
							Date:        "2024-01-15",
							AccountRef:  "@moneyAccounts.mainCash",
							CategoryRef: "@paymentCategories.salesRevenue",
							Comment:     "Sample payment",
						},
					},
				},
				CRM: &schemas.CRMSpec{
					Clients: []schemas.ClientSpec{
						{
							FirstName: "John",
							LastName:  "Doe",
							Email:     "john.doe@example.com",
							Phone:     "+1234567890",
							Ref:       "johnDoe",
						},
					},
				},
				Warehouse: &schemas.WarehouseSpec{
					Units: []schemas.UnitSpec{
						{
							Title:      "Pieces",
							ShortTitle: "pcs",
							Ref:        "pieces",
						},
					},
					Products: []schemas.ProductSpec{
						{
							Name:    "Test Product",
							UnitRef: "@units.pieces",
							Price:   50.00,
							Ref:     "testProduct",
						},
					},
				},
			},
			Options: &schemas.OptionsSpec{
				ClearExisting:      false,
				ReturnIds:          true,
				ValidateReferences: true,
				StopOnError:        true,
			},
		},
	}

	scenario, exists := scenarios[name]
	return scenario, exists
}
