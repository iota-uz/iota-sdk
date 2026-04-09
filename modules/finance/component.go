// Package finance provides this package.
package finance

import (
	"embed"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
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

	uploadRepo := corepersistence.NewUploadRepository()
	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		persistence.NewTransactionRepository(),
		ctx.EventPublisher(),
	)
	transactionRepo := persistence.NewTransactionRepository()
	categoryRepo := persistence.NewExpenseCategoryRepository()
	transactionService := services.NewTransactionService(transactionRepo, ctx.EventPublisher())
	paymentService := services.NewPaymentService(
		persistence.NewPaymentRepository(),
		ctx.EventPublisher(),
		moneyAccountService,
		uploadRepo,
	)
	expenseCategoryService := services.NewExpenseCategoryService(categoryRepo, ctx.EventPublisher())
	paymentCategoryService := services.NewPaymentCategoryService(
		persistence.NewPaymentCategoryRepository(),
		ctx.EventPublisher(),
	)
	expenseService := services.NewExpenseService(
		persistence.NewExpenseRepository(categoryRepo, transactionRepo),
		ctx.EventPublisher(),
		moneyAccountService,
		uploadRepo,
	)
	counterpartyService := services.NewCounterpartyService(persistence.NewCounterpartyRepository())
	inventoryService := services.NewInventoryService(persistence.NewInventoryRepository())
	debtService := services.NewDebtService(persistence.NewDebtRepository(), ctx.EventPublisher())
	financialReportService := services.NewFinancialReportService(
		query.NewPgFinancialReportsQueryRepository(),
		ctx.EventPublisher(),
	)

	composition.Provide[*services.TransactionService](builder, transactionService)
	composition.Provide[*services.PaymentService](builder, paymentService)
	composition.Provide[*services.ExpenseCategoryService](builder, expenseCategoryService)
	composition.Provide[*services.PaymentCategoryService](builder, paymentCategoryService)
	composition.Provide[*services.ExpenseService](builder, expenseService)
	composition.Provide[*services.MoneyAccountService](builder, moneyAccountService)
	composition.Provide[*services.CounterpartyService](builder, counterpartyService)
	composition.Provide[*services.InventoryService](builder, inventoryService)
	composition.Provide[*services.DebtService](builder, debtService)
	composition.Provide[*services.FinancialReportService](builder, financialReportService)

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
