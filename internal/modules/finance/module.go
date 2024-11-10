package finance

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/internal/modules/finance/controllers"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Assets() *hashfs.FS {
	return nil
}

func (m *Module) Seed(ctx context.Context) error {
	return nil
}

func (m *Module) Name() string {
	return "finance"
}

func (m *Module) NavigationItems() []shared.NavigationItem {
	return []shared.NavigationItem{
		{
			Name:     "Users",
			Children: nil,
			Icon:     icons.Users(icons.Props{Size: "20"}),
			Href:     "/users",
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
	}
}
