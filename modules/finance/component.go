// Package finance provides this package.
package finance

import (
	"embed"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
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
	composition.AddLocales(builder, &localeFiles)
	composition.AddNavItems(builder, NavItems...)
	composition.AddQuickLinks(builder,
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
	)

	composition.ProvideFunc(builder, persistence.NewMoneyAccountRepository)
	composition.ProvideFunc(builder, persistence.NewTransactionRepository)
	composition.ProvideFunc(builder, persistence.NewExpenseCategoryRepository)
	composition.ProvideFunc(builder, persistence.NewPaymentCategoryRepository)
	composition.ProvideFunc(builder, persistence.NewPaymentRepository)
	composition.ProvideFunc(builder, persistence.NewExpenseRepository)
	composition.ProvideFunc(builder, persistence.NewCounterpartyRepository)
	composition.ProvideFunc(builder, persistence.NewInventoryRepository)
	composition.ProvideFunc(builder, persistence.NewDebtRepository)
	composition.ProvideFunc(builder, query.NewPgFinancialReportsQueryRepository)

	composition.ProvideFunc(builder, services.NewMoneyAccountService)
	composition.ProvideFunc(builder, services.NewTransactionService)
	composition.ProvideFunc(builder, services.NewPaymentService)
	composition.ProvideFunc(builder, services.NewExpenseCategoryService)
	composition.ProvideFunc(builder, services.NewPaymentCategoryService)
	composition.ProvideFunc(builder, services.NewExpenseService)
	composition.ProvideFunc(builder, services.NewCounterpartyService)
	composition.ProvideFunc(builder, services.NewInventoryService)
	composition.ProvideFunc(builder, services.NewDebtService)
	composition.ProvideFunc(builder, services.NewFinancialReportService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, financeControllers)
	}
	return nil
}

func financeControllers(
	moneyAccountService *services.MoneyAccountService,
	transactionService *services.TransactionService,
	paymentService *services.PaymentService,
	expenseCategoryService *services.ExpenseCategoryService,
	paymentCategoryService *services.PaymentCategoryService,
	counterpartyService *services.CounterpartyService,
	inventoryService *services.InventoryService,
	debtService *services.DebtService,
	financialReportService *services.FinancialReportService,
	currencyService *coreservices.CurrencyService,
	reportsQueryRepo query.FinancialReportsQueryRepository,
) []application.Controller {
	return []application.Controller{
		controllers.NewFinancialOverviewController(paymentService, moneyAccountService, counterpartyService, paymentCategoryService, transactionService),
		controllers.NewMoneyAccountController(moneyAccountService, transactionService, currencyService),
		controllers.NewExpenseCategoriesController(expenseCategoryService),
		controllers.NewPaymentCategoriesController(paymentCategoryService),
		controllers.NewCounterpartiesController(counterpartyService),
		controllers.NewInventoryController(inventoryService, currencyService),
		controllers.NewDebtsController(debtService, counterpartyService, transactionService),
		controllers.NewDebtAggregateController(debtService, counterpartyService),
		controllers.NewFinancialReportController(financialReportService, reportsQueryRepo),
		controllers.NewCashflowController(financialReportService, moneyAccountService, reportsQueryRepo),
	}
}
