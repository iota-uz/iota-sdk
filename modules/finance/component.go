// Package finance provides this package.
package finance

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	debt "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	expense "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	category "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyaccount "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	payment "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	counterparty "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	inventory "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	transaction "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "finance",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	ctx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{
			spotlight.NewQuickLink(ExpenseCategoriesItem.Name, ExpenseCategoriesItem.Href),
			spotlight.NewQuickLink(PaymentCategoriesItem.Name, PaymentCategoriesItem.Href),
			spotlight.NewQuickLink(PaymentsItem.Name, "/finance/overview?tab=payments"),
			spotlight.NewQuickLink(ExpensesItem.Name, "/finance/overview?tab=expenses"),
			spotlight.NewQuickLink(DebtsItem.Name, DebtsItem.Href),
			spotlight.NewQuickLink(AccountsItem.Name, AccountsItem.Href),
			spotlight.NewQuickLink(InventoryItem.Name, InventoryItem.Href),
			spotlight.NewQuickLink("NavigationLinks.IncomeStatement", "/finance/reports/income-statement"),
			spotlight.NewQuickLink("NavigationLinks.CashflowStatement", "/finance/reports/cashflow"),
			spotlight.NewQuickLink("Expenses.List.New", "/finance/overview?tab=expenses"),
			spotlight.NewQuickLink("MoneyAccounts.List.New", "/finance/accounts/new"),
			spotlight.NewQuickLink("Payments.List.New", "/finance/overview?tab=payments"),
			spotlight.NewQuickLink("ExpenseCategories.List.New", "/finance/expense-categories/new"),
			spotlight.NewQuickLink("PaymentCategories.List.New", "/finance/payment-categories/new"),
			spotlight.NewQuickLink("Inventory.List.New", "/finance/inventory/new"),
		}, nil
	})

	uploadRepo := composition.Use[upload.Repository]()
	moneyAccountRepo := composition.Use[moneyaccount.Repository]()
	transactionRepo := composition.Use[transaction.Repository]()
	expenseCategoryRepo := composition.Use[category.Repository]()
	paymentCategoryRepo := composition.Use[paymentcategory.Repository]()
	paymentRepo := composition.Use[payment.Repository]()
	expenseRepo := composition.Use[expense.Repository]()
	counterpartyRepo := composition.Use[counterparty.Repository]()
	inventoryRepo := composition.Use[inventory.Repository]()
	debtRepo := composition.Use[debt.Repository]()
	reportQueryRepo := composition.Use[query.FinancialReportsQueryRepository]()
	moneyAccountService := composition.Use[*services.MoneyAccountService]()

	composition.Provide[moneyaccount.Repository](builder, func() moneyaccount.Repository {
		return persistence.NewMoneyAccountRepository()
	})
	composition.Provide[transaction.Repository](builder, func() transaction.Repository {
		return persistence.NewTransactionRepository()
	})
	composition.Provide[category.Repository](builder, func() category.Repository {
		return persistence.NewExpenseCategoryRepository()
	})
	composition.Provide[paymentcategory.Repository](builder, func() paymentcategory.Repository {
		return persistence.NewPaymentCategoryRepository()
	})
	composition.Provide[payment.Repository](builder, func() payment.Repository {
		return persistence.NewPaymentRepository()
	})
	composition.Provide[expense.Repository](builder, func(container *composition.Container) (expense.Repository, error) {
		resolvedCategoryRepo, err := expenseCategoryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedTransactionRepo, err := transactionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return persistence.NewExpenseRepository(resolvedCategoryRepo, resolvedTransactionRepo), nil
	})
	composition.Provide[counterparty.Repository](builder, func() counterparty.Repository {
		return persistence.NewCounterpartyRepository()
	})
	composition.Provide[inventory.Repository](builder, func() inventory.Repository {
		return persistence.NewInventoryRepository()
	})
	composition.Provide[debt.Repository](builder, func() debt.Repository {
		return persistence.NewDebtRepository()
	})
	composition.Provide[query.FinancialReportsQueryRepository](builder, func() query.FinancialReportsQueryRepository {
		return query.NewPgFinancialReportsQueryRepository()
	})
	composition.Provide[*services.MoneyAccountService](builder, func(container *composition.Container) (*services.MoneyAccountService, error) {
		resolvedMoneyAccountRepo, err := moneyAccountRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedTransactionRepo, err := transactionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewMoneyAccountService(resolvedMoneyAccountRepo, resolvedTransactionRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.TransactionService](builder, func(container *composition.Container) (*services.TransactionService, error) {
		resolvedTransactionRepo, err := transactionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewTransactionService(resolvedTransactionRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.PaymentService](builder, func(container *composition.Container) (*services.PaymentService, error) {
		resolvedPaymentRepo, err := paymentRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedMoneyAccountService, err := moneyAccountService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUploadRepo, err := uploadRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewPaymentService(resolvedPaymentRepo, ctx.EventPublisher(), resolvedMoneyAccountService, resolvedUploadRepo), nil
	})
	composition.Provide[*services.ExpenseCategoryService](builder, func(container *composition.Container) (*services.ExpenseCategoryService, error) {
		resolvedCategoryRepo, err := expenseCategoryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewExpenseCategoryService(resolvedCategoryRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.PaymentCategoryService](builder, func(container *composition.Container) (*services.PaymentCategoryService, error) {
		resolvedPaymentCategoryRepo, err := paymentCategoryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewPaymentCategoryService(resolvedPaymentCategoryRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.ExpenseService](builder, func(container *composition.Container) (*services.ExpenseService, error) {
		resolvedExpenseRepo, err := expenseRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedMoneyAccountService, err := moneyAccountService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUploadRepo, err := uploadRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewExpenseService(resolvedExpenseRepo, ctx.EventPublisher(), resolvedMoneyAccountService, resolvedUploadRepo), nil
	})
	composition.Provide[*services.CounterpartyService](builder, func(container *composition.Container) (*services.CounterpartyService, error) {
		resolvedCounterpartyRepo, err := counterpartyRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewCounterpartyService(resolvedCounterpartyRepo), nil
	})
	composition.Provide[*services.InventoryService](builder, func(container *composition.Container) (*services.InventoryService, error) {
		resolvedInventoryRepo, err := inventoryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewInventoryService(resolvedInventoryRepo), nil
	})
	composition.Provide[*services.DebtService](builder, func(container *composition.Container) (*services.DebtService, error) {
		resolvedDebtRepo, err := debtRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewDebtService(resolvedDebtRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.FinancialReportService](builder, func(container *composition.Container) (*services.FinancialReportService, error) {
		resolvedReportQueryRepo, err := reportQueryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewFinancialReportService(resolvedReportQueryRepo, ctx.EventPublisher()), nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			return []application.Controller{
				controllers.NewFinancialOverviewController(app),
				controllers.NewMoneyAccountController(app),
				controllers.NewExpenseCategoriesController(app),
				controllers.NewPaymentCategoriesController(app),
				controllers.NewCounterpartiesController(app),
				controllers.NewInventoryController(app),
				controllers.NewDebtsController(app),
				controllers.NewDebtAggregateController(app),
				controllers.NewFinancialReportController(app),
				controllers.NewCashflowController(app),
			}, nil
		})
	}

	return nil
}
