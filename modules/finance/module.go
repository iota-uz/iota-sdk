package finance

import (
	"embed"
	"github.com/iota-agency/iota-sdk/modules/finance/controllers"
	"github.com/iota-agency/iota-sdk/modules/finance/permissions"
	"github.com/iota-agency/iota-sdk/modules/finance/persistence"
	"github.com/iota-agency/iota-sdk/modules/finance/services"
	"github.com/iota-agency/iota-sdk/modules/finance/templates"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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
	app.RegisterServices(
		services.NewPaymentService(
			persistence.NewPaymentRepository(),
			app.EventPublisher(),
			moneyAccountService,
		),
	)
	app.RegisterServices(
		services.NewExpenseCategoryService(
			persistence.NewExpenseCategoryRepository(),
			app.EventPublisher(),
		))
	app.RegisterServices(
		services.NewExpenseService(
			persistence.NewExpenseRepository(),
			app.EventPublisher(),
			moneyAccountService,
		),
	)
	app.RegisterServices(moneyAccountService)

	app.RegisterControllers(
		controllers.NewExpensesController(app),
		controllers.NewMoneyAccountController(app),
		controllers.NewExpenseCategoriesController(app),
		controllers.NewPaymentsController(app),
	)
	app.RegisterPermissions(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterMigrationDirs(&migrationFiles)
	app.RegisterModule(m)
	return nil
}

func (m *Module) Name() string {
	return "finance"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Finances"}),
			Href: "/finance",
			Icon: icons.Money(icons.Props{Size: "20"}),
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.ExpenseCategories"}),
					Href:        "/finance/expense-categories",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Payments"}),
					Href:        "/finance/payments",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Expenses"}),
					Href:        "/finance/expenses",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Accounts"}),
					Href:        "/finance/accounts",
					Permissions: nil,
				},
			},
		},
	}
}
