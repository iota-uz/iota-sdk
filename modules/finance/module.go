package finance

import (
	"context"
	"embed"
	"github.com/iota-agency/iota-sdk/modules/finance/controllers"
	"github.com/iota-agency/iota-sdk/modules/finance/templates"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

//go:embed locales/*.json
var localeFiles embed.FS

////go:embed migrations/*.sql
//var migrationFiles embed.FS

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app *application.Application) error {
	return nil
}

func (m *Module) MigrationDirs() *embed.FS {
	//return &migrationFiles
	return nil
}

func (m *Module) Assets() *embed.FS {
	return nil
}

func (m *Module) Templates() *embed.FS {
	return &templates.FS
}

func (m *Module) Seed(ctx context.Context, app *application.Application) error {
	return nil
}

func (m *Module) Name() string {
	return "finance"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Finances"}),
			Href: "#",
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

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		controllers.NewExpensesController,
		controllers.NewMoneyAccountController,
		controllers.NewExpenseCategoriesController,
		controllers.NewPaymentsController,
	}
}

func (m *Module) LocaleFiles() *embed.FS {
	return &localeFiles
}