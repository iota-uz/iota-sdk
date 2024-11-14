package finance

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/finance/controllers"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app *application.Application) error {
	return nil
}

func (m *Module) MigrationDirs() []string {
	return []string{
		"internal/modules/finance/migrations",
	}
}

func (m *Module) Migrations() []string {
	return []string{
		"internal/modules/finance/migrations",
	}
}

func (m *Module) Assets() *hashfs.FS {
	return nil
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

func (m *Module) LocaleFiles() []string {
	return []string{
		"internal/modules/finance/locales/en.json",
		"internal/modules/finance/locales/ru.json",
		"internal/modules/finance/locales/uz.json",
	}
}
