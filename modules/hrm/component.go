// Package hrm provides this package.
package hrm

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/hrm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/hrm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/hrm-schema.sql
var MigrationFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "hrm",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddLocales(builder, &LocaleFiles)
	composition.AddNavItems(builder, NavItems...)
	composition.AddQuickLinks(builder, spotlight.NewQuickLink(EmployeesLink.Name, EmployeesLink.Href))
	composition.ContributeMigrations(builder, &MigrationFiles)

	composition.ProvideFunc(builder, persistence.NewPositionRepository)
	composition.ProvideFunc(builder, persistence.NewEmployeeRepository)
	composition.ProvideFunc(builder, services.NewPositionService)
	composition.ProvideFunc(builder, services.NewEmployeeService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(app application.Application, employeeService *services.EmployeeService) application.Controller {
			return controllers.NewEmployeeController(app, employeeService)
		})
	}
	return nil
}
