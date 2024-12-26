package finance

import (
	"embed"
	corepersistence "github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-agency/iota-sdk/modules/finance/controllers"
	"github.com/iota-agency/iota-sdk/modules/finance/permissions"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence"
	"github.com/iota-agency/iota-sdk/modules/finance/services"
	"github.com/iota-agency/iota-sdk/modules/finance/templates"
	"github.com/iota-agency/iota-sdk/pkg/application"
)

//go:embed locales/*.json
var localeFiles embed.FS

//go:embed migrations/*.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RegisterTemplates(&templates.FS)

	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		app.EventPublisher(),
	)
	currencyRepo := corepersistence.NewCurrencyRepository()
	app.RegisterServices(
		services.NewPaymentService(
			persistence.NewPaymentRepository(),
			app.EventPublisher(),
			moneyAccountService,
		),
		services.NewExpenseCategoryService(
			persistence.NewExpenseCategoryRepository(currencyRepo),
			app.EventPublisher(),
		),
		services.NewExpenseService(
			persistence.NewExpenseRepository(),
			app.EventPublisher(),
			moneyAccountService,
		),
		moneyAccountService,
	)

	app.RegisterControllers(
		controllers.NewExpensesController(app),
		controllers.NewMoneyAccountController(app),
		controllers.NewExpenseCategoriesController(app),
		controllers.NewPaymentsController(app),
	)
	app.RegisterPermissions(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterMigrationDirs(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "finance"
}
