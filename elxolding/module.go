package elxolding

import (
	"context"
	"fmt"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/elxolding/assets"
	"github.com/iota-agency/iota-erp/elxolding/controllers"
	"github.com/iota-agency/iota-erp/elxolding/seed"
	"github.com/iota-agency/iota-erp/elxolding/services"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	persistence2 "github.com/iota-agency/iota-erp/internal/modules/warehouse/persistence"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

var (
	ProjectDir = "elxolding"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app *application.Application) error {
	dashboardService := services.NewDashboardService(
		persistence2.NewPositionRepository(),
		persistence2.NewProductRepository(),
		persistence2.NewOrderRepository(),
	)
	app.RegisterService(dashboardService)
	return nil
}

func (m *Module) MigrationDirs() []string {
	return []string{
		//fmt.Sprintf("%s/migrations", ProjectDir),
	}
}

func (m *Module) Assets() *hashfs.FS {
	return assets.FS
}

func (m *Module) Seed(ctx context.Context, app *application.Application) error {
	seedFuncs := []shared.SeedFunc{
		seed.SeedUser,
		seed.SeedPositions,
		seed.SeedProducts,
	}
	for _, seedFunc := range seedFuncs {
		if err := seedFunc(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) Name() string {
	return "elxolding"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Users"}),
			Children: nil,
			Icon:     icons.Users(icons.Props{Size: "20"}),
			Href:     "/users",
		},
	}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		controllers.NewUsersController,
		controllers.NewLoginController,
		controllers.NewAccountController,
		controllers.NewDashboardController,
	}
}

func (m *Module) LocaleFiles() []string {
	return []string{
		fmt.Sprintf("%s/locales/en.json", ProjectDir),
		fmt.Sprintf("%s/locales/ru.json", ProjectDir),
		fmt.Sprintf("%s/locales/uz.json", ProjectDir),
	}
}
