package services

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/testkit/domain/schemas"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type PopulateService struct {
	app             application.Application
	referenceMap    map[string]interface{}
	createdEntities map[string]interface{}
}

func NewPopulateService(app application.Application) *PopulateService {
	return &PopulateService{
		app:             app,
		referenceMap:    make(map[string]interface{}),
		createdEntities: make(map[string]interface{}),
	}
}

func (s *PopulateService) Execute(ctx context.Context, req *schemas.PopulateRequest) (map[string]interface{}, error) {
	logger := composables.UseLogger(ctx)
	db := s.app.DB()

	// Reset state for new population request
	s.referenceMap = make(map[string]interface{})
	s.createdEntities = make(map[string]interface{})

	// Begin transaction
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Add transaction to context
	ctxWithTx := composables.WithTx(ctx, tx)

	logger.Info("Starting data population")

	// Handle tenant setup
	if req.Tenant != nil {
		if err := s.setupTenant(ctxWithTx, req.Tenant); err != nil {
			return nil, fmt.Errorf("failed to setup tenant: %w", err)
		}
	}

	// Populate data if provided
	if req.Data != nil {
		if err := s.populateData(ctxWithTx, req.Data); err != nil {
			return nil, fmt.Errorf("failed to populate data: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctxWithTx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info("Data population completed successfully")

	// Return created entities if requested
	if req.Options != nil && req.Options.ReturnIds {
		return s.createdEntities, nil
	}

	return map[string]interface{}{
		"message": "Data populated successfully",
	}, nil
}

func (s *PopulateService) setupTenant(ctx context.Context, tenant *schemas.TenantSpec) error {
	logger := composables.UseLogger(ctx)

	// TODO: Implement tenant creation
	// For now, we'll use the default tenant setup similar to seed command
	logger.WithField("tenantName", tenant.Name).Debug("Setting up tenant")

	return nil
}

func (s *PopulateService) populateData(ctx context.Context, data *schemas.DataSpec) error {
	// Create users first as they're referenced by other entities
	if len(data.Users) > 0 {
		if err := s.createUsers(ctx, data.Users); err != nil {
			return fmt.Errorf("failed to create users: %w", err)
		}
	}

	// Create finance entities
	if data.Finance != nil {
		if err := s.createFinanceData(ctx, data.Finance); err != nil {
			return fmt.Errorf("failed to create finance data: %w", err)
		}
	}

	// Create CRM entities
	if data.CRM != nil {
		if err := s.createCRMData(ctx, data.CRM); err != nil {
			return fmt.Errorf("failed to create CRM data: %w", err)
		}
	}

	// Create warehouse entities
	if data.Warehouse != nil {
		if err := s.createWarehouseData(ctx, data.Warehouse); err != nil {
			return fmt.Errorf("failed to create warehouse data: %w", err)
		}
	}

	return nil
}

func (s *PopulateService) createUsers(ctx context.Context, users []schemas.UserSpec) error {
	logger := composables.UseLogger(ctx)

	for _, user := range users {
		logger.WithField("email", user.Email).Debug("Creating user")

		// TODO: Implement user creation using core module services
		// This would involve:
		// 1. Creating user aggregate
		// 2. Setting password
		// 3. Assigning permissions
		// 4. Saving to database

		// For now, store the reference for later resolution
		if user.Ref != "" {
			s.referenceMap["users."+user.Ref] = map[string]interface{}{
				"email":     user.Email,
				"firstName": user.FirstName,
				"lastName":  user.LastName,
			}
		}

		// Track created entity
		s.createdEntities["users"] = append(
			s.getSliceFromMap("users"),
			map[string]interface{}{
				"email": user.Email,
				"ref":   user.Ref,
			},
		)
	}

	return nil
}

func (s *PopulateService) createFinanceData(ctx context.Context, finance *schemas.FinanceSpec) error {
	// Create money accounts
	if len(finance.MoneyAccounts) > 0 {
		if err := s.createMoneyAccounts(ctx, finance.MoneyAccounts); err != nil {
			return fmt.Errorf("failed to create money accounts: %w", err)
		}
	}

	// Create payment categories
	if len(finance.PaymentCategories) > 0 {
		if err := s.createPaymentCategories(ctx, finance.PaymentCategories); err != nil {
			return fmt.Errorf("failed to create payment categories: %w", err)
		}
	}

	// Create expense categories
	if len(finance.ExpenseCategories) > 0 {
		if err := s.createExpenseCategories(ctx, finance.ExpenseCategories); err != nil {
			return fmt.Errorf("failed to create expense categories: %w", err)
		}
	}

	// Create counterparties
	if len(finance.Counterparties) > 0 {
		if err := s.createCounterparties(ctx, finance.Counterparties); err != nil {
			return fmt.Errorf("failed to create counterparties: %w", err)
		}
	}

	// Create payments
	if len(finance.Payments) > 0 {
		if err := s.createPayments(ctx, finance.Payments); err != nil {
			return fmt.Errorf("failed to create payments: %w", err)
		}
	}

	// Create expenses
	if len(finance.Expenses) > 0 {
		if err := s.createExpenses(ctx, finance.Expenses); err != nil {
			return fmt.Errorf("failed to create expenses: %w", err)
		}
	}

	// Create debts
	if len(finance.Debts) > 0 {
		if err := s.createDebts(ctx, finance.Debts); err != nil {
			return fmt.Errorf("failed to create debts: %w", err)
		}
	}

	return nil
}

func (s *PopulateService) createMoneyAccounts(ctx context.Context, accounts []schemas.MoneyAccountSpec) error {
	logger := composables.UseLogger(ctx)

	for _, account := range accounts {
		logger.WithField("name", account.Name).Debug("Creating money account")

		// TODO: Implement money account creation using finance module

		if account.Ref != "" {
			s.referenceMap["moneyAccounts."+account.Ref] = map[string]interface{}{
				"name":     account.Name,
				"currency": account.Currency,
				"type":     account.Type,
			}
		}

		s.createdEntities["moneyAccounts"] = append(
			s.getSliceFromMap("moneyAccounts"),
			map[string]interface{}{
				"name": account.Name,
				"ref":  account.Ref,
			},
		)
	}

	return nil
}

func (s *PopulateService) createPaymentCategories(ctx context.Context, categories []schemas.PaymentCategorySpec) error {
	logger := composables.UseLogger(ctx)

	for _, category := range categories {
		logger.WithField("name", category.Name).Debug("Creating payment category")

		// TODO: Implement payment category creation

		if category.Ref != "" {
			s.referenceMap["paymentCategories."+category.Ref] = map[string]interface{}{
				"name": category.Name,
				"type": category.Type,
			}
		}

		s.createdEntities["paymentCategories"] = append(
			s.getSliceFromMap("paymentCategories"),
			map[string]interface{}{
				"name": category.Name,
				"ref":  category.Ref,
			},
		)
	}

	return nil
}

func (s *PopulateService) createExpenseCategories(ctx context.Context, categories []schemas.ExpenseCategorySpec) error {
	logger := composables.UseLogger(ctx)

	for _, category := range categories {
		logger.WithField("name", category.Name).Debug("Creating expense category")

		// TODO: Implement expense category creation

		if category.Ref != "" {
			s.referenceMap["expenseCategories."+category.Ref] = map[string]interface{}{
				"name": category.Name,
				"type": category.Type,
			}
		}

		s.createdEntities["expenseCategories"] = append(
			s.getSliceFromMap("expenseCategories"),
			map[string]interface{}{
				"name": category.Name,
				"ref":  category.Ref,
			},
		)
	}

	return nil
}

func (s *PopulateService) createCounterparties(ctx context.Context, counterparties []schemas.CounterpartySpec) error {
	// TODO: Implement counterparty creation
	return nil
}

func (s *PopulateService) createPayments(ctx context.Context, payments []schemas.PaymentSpec) error {
	// TODO: Implement payment creation with reference resolution
	return nil
}

func (s *PopulateService) createExpenses(ctx context.Context, expenses []schemas.ExpenseSpec) error {
	// TODO: Implement expense creation with reference resolution
	return nil
}

func (s *PopulateService) createDebts(ctx context.Context, debts []schemas.DebtSpec) error {
	// TODO: Implement debt creation
	return nil
}

func (s *PopulateService) createCRMData(ctx context.Context, crm *schemas.CRMSpec) error {
	if len(crm.Clients) > 0 {
		// TODO: Implement client creation
	}
	return nil
}

func (s *PopulateService) createWarehouseData(ctx context.Context, warehouse *schemas.WarehouseSpec) error {
	if len(warehouse.Units) > 0 {
		// TODO: Implement unit creation
	}
	if len(warehouse.Products) > 0 {
		// TODO: Implement product creation
	}
	return nil
}

func (s *PopulateService) resolveReference(ref string) (interface{}, error) {
	// Handle @category.name syntax
	if len(ref) > 0 && ref[0] == '@' {
		ref = ref[1:]
	}

	entity, exists := s.referenceMap[ref]
	if !exists {
		return nil, fmt.Errorf("reference '%s' not found", ref)
	}

	return entity, nil
}

func (s *PopulateService) getSliceFromMap(key string) []interface{} {
	if existing, exists := s.createdEntities[key]; exists {
		if slice, ok := existing.([]interface{}); ok {
			return slice
		}
	}
	return []interface{}{}
}
