package superadmin

import (
	"embed"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

type ModuleOptions struct {
	// TODO: Add module options as needed
}

func NewModule(opts *ModuleOptions) application.Module {
	if opts == nil {
		opts = &ModuleOptions{}
	}
	return &Module{
		options: opts,
	}
}

type Module struct {
	options *ModuleOptions
}

func (m *Module) Register(app application.Application) error {
	// Register locale files
	app.RegisterLocaleFiles(&LocaleFiles)

	// Register repositories
	analyticsRepo := persistence.NewPgAnalyticsQueryRepository()

	// User repository for tenant users service
	uploadRepo := corepersistence.NewUploadRepository()
	userRepo := corepersistence.NewUserRepository(uploadRepo)

	// Register services
	app.RegisterServices(
		services.NewAnalyticsService(analyticsRepo),
		services.NewTenantService(analyticsRepo),
		services.NewTenantUsersService(userRepo),
	)

	// Register controllers
	app.RegisterControllers(
		controllers.NewDashboardController(app),
		controllers.NewTenantsController(app),
	)

	return nil
}

func (m *Module) Name() string {
	return "superadmin"
}
