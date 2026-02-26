package finance

import (
	"embed"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/finance-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	_ = migrationFiles

	// Create upload repository for attachment functionality
	uploadRepo := corepersistence.NewUploadRepository()

	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		persistence.NewTransactionRepository(),
		app.EventPublisher(),
	)
	transactionRepo := persistence.NewTransactionRepository()
	categoryRepo := persistence.NewExpenseCategoryRepository()
	app.RegisterServices(
		services.NewTransactionService(
			transactionRepo,
			app.EventPublisher(),
		),
		services.NewPaymentService(
			persistence.NewPaymentRepository(),
			app.EventPublisher(),
			moneyAccountService,
			uploadRepo,
		),
		services.NewExpenseCategoryService(
			categoryRepo,
			app.EventPublisher(),
		),
		services.NewPaymentCategoryService(
			persistence.NewPaymentCategoryRepository(),
			app.EventPublisher(),
		),
		services.NewExpenseService(
			persistence.NewExpenseRepository(categoryRepo, transactionRepo),
			app.EventPublisher(),
			moneyAccountService,
			uploadRepo,
		),
		moneyAccountService,
		services.NewCounterpartyService(persistence.NewCounterpartyRepository()),
		services.NewInventoryService(persistence.NewInventoryRepository()),
		services.NewDebtService(
			persistence.NewDebtRepository(),
			app.EventPublisher(),
		),
		services.NewFinancialReportService(
			query.NewPgFinancialReportsQueryRepository(),
			app.EventPublisher(),
		),
	)

	app.RegisterControllers(
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
	)
	app.QuickLinks().Add(
		spotlight.NewQuickLink(ExpenseCategoriesItem.Name, ExpenseCategoriesItem.Href),
		spotlight.NewQuickLink(PaymentCategoriesItem.Name, PaymentCategoriesItem.Href),
		spotlight.NewQuickLink(PaymentsItem.Name, "/finance/overview?tab=payments"),
		spotlight.NewQuickLink(ExpensesItem.Name, "/finance/overview?tab=expenses"),
		spotlight.NewQuickLink(DebtsItem.Name, DebtsItem.Href),
		spotlight.NewQuickLink(AccountsItem.Name, AccountsItem.Href),
		spotlight.NewQuickLink(InventoryItem.Name, InventoryItem.Href),
		spotlight.NewQuickLink("NavigationLinks.IncomeStatement",
			"/finance/reports/income-statement",
		),
		spotlight.NewQuickLink("NavigationLinks.CashflowStatement",
			"/finance/reports/cashflow",
		),
		spotlight.NewQuickLink("Expenses.List.New",
			"/finance/overview?tab=expenses",
		),
		spotlight.NewQuickLink("MoneyAccounts.List.New",
			"/finance/accounts/new",
		),
		spotlight.NewQuickLink("Payments.List.New",
			"/finance/overview?tab=payments",
		),
		spotlight.NewQuickLink("ExpenseCategories.List.New",
			"/finance/expense-categories/new",
		),
		spotlight.NewQuickLink("PaymentCategories.List.New",
			"/finance/payment-categories/new",
		),
		spotlight.NewQuickLink("Inventory.List.New",
			"/finance/inventory/new",
		),
	)

	app.RegisterLocaleFiles(&localeFiles)
	return nil
}

func (m *Module) Name() string {
	return "finance"
}
