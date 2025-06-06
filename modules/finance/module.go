package finance

import (
	"embed"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
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
	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		persistence.NewTransactionRepository(),
		app.EventPublisher(),
	)
	transactionRepo := persistence.NewTransactionRepository()
	categoryRepo := persistence.NewExpenseCategoryRepository()
	app.RegisterServices(
		services.NewPaymentService(
			persistence.NewPaymentRepository(),
			app.EventPublisher(),
			moneyAccountService,
		),
		services.NewExpenseCategoryService(
			categoryRepo,
			app.EventPublisher(),
		),
		services.NewExpenseService(
			persistence.NewExpenseRepository(categoryRepo, transactionRepo),
			app.EventPublisher(),
			moneyAccountService,
		),
		moneyAccountService,
		services.NewCounterpartyService(persistence.NewCounterpartyRepository()),
	)

	app.RegisterControllers(
		controllers.NewExpensesController(app),
		controllers.NewMoneyAccountController(app),
		controllers.NewExpenseCategoriesController(app),
		controllers.NewPaymentsController(app),
		controllers.NewCounterpartiesController(app),
	)
	app.QuickLinks().Add(
		spotlight.NewQuickLink(nil, ExpenseCategoriesItem.Name, ExpenseCategoriesItem.Href),
		spotlight.NewQuickLink(nil, PaymentsItem.Name, PaymentsItem.Href),
		spotlight.NewQuickLink(nil, ExpensesItem.Name, ExpensesItem.Href),
		spotlight.NewQuickLink(nil, AccountsItem.Name, AccountsItem.Href),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Expenses.List.New",
			"/finance/expenses/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Accounts.List.New",
			"/finance/accounts/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Payments.List.New",
			"/finance/payments/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"ExpenseCategories.List.New",
			"/finance/expense-categories/new",
		),
	)

	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.Migrations().RegisterSchema(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "finance"
}
