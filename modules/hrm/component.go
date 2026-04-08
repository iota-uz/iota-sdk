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
	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	positionService := services.NewPositionService(persistence.NewPositionRepository(), app.EventPublisher())
	employeeService := services.NewEmployeeService(persistence.NewEmployeeRepository(), app.EventPublisher())
	composition.Provide[*services.PositionService](builder, positionService)
	composition.Provide[*services.EmployeeService](builder, employeeService)
	app.QuickLinks().Add(spotlight.NewQuickLink(EmployeesLink.Name, EmployeesLink.Href))

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			return []application.Controller{controllers.NewEmployeeController(app)}, nil
		})
	}

	return nil
}
