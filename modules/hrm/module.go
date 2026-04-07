// Package hrm provides this package.
package hrm

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/hrm-schema.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) RegisterWiring(app application.Application) error {
	app.RegisterLocaleFiles(&LocaleFiles)
	app.RegisterServices(
		services.NewPositionService(persistence.NewPositionRepository(), app.EventPublisher()),
		services.NewEmployeeService(persistence.NewEmployeeRepository(), app.EventPublisher()),
	)
	app.QuickLinks().Add(
		spotlight.NewQuickLink(EmployeesLink.Name, EmployeesLink.Href),
	)
	return nil
}

func (m *Module) RegisterTransports(app application.Application) error {
	app.RegisterControllers(
		controllers.NewEmployeeController(app),
	)
	return nil
}

func (m *Module) Name() string {
	return "hrm"
}
