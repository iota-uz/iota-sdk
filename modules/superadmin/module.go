// Package superadmin provides this package.
package superadmin

import (
	"embed"
	"fmt"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

type ModuleOptions struct {
	// Module currently has no configuration options
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

func (m *Module) RegisterWiring(app application.Application) error {
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

	return nil
}

func (m *Module) RegisterTransports(app application.Application) error {
	const op serrors.Op = "superadmin.Module.RegisterTransports"

	userServiceAny := app.Service(coreservices.UserService{})
	userService, ok := userServiceAny.(*coreservices.UserService)
	if !ok || userService == nil {
		return serrors.E(op, fmt.Errorf("user service is not registered"))
	}

	// Register controllers
	app.RegisterControllers(
		controllers.NewDashboardController(app),
		controllers.NewTenantsController(app, userService),
	)
	return nil
}

func (m *Module) Name() string {
	return "superadmin"
}
