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
	"github.com/iota-uz/iota-sdk/pkg/types"
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
	ctx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{spotlight.NewQuickLink(EmployeesLink.Name, EmployeesLink.Href)}, nil
	})

	positionService := services.NewPositionService(persistence.NewPositionRepository(), ctx.EventPublisher())
	employeeService := services.NewEmployeeService(persistence.NewEmployeeRepository(), ctx.EventPublisher())
	composition.Provide[*services.PositionService](builder, positionService)
	composition.Provide[*services.EmployeeService](builder, employeeService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			return []application.Controller{controllers.NewEmployeeController(app)}, nil
		})
	}

	return nil
}
