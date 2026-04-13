// Package superadmin provides this package.
package superadmin

import (
	"embed"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/superadmin/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/superadmin/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/superadmin/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

type ModuleOptions struct {
	// Module currently has no configuration options
}

func NewComponent(opts *ModuleOptions) composition.Component {
	if opts == nil {
		opts = &ModuleOptions{}
	}
	return &component{options: opts}
}

type component struct {
	options *ModuleOptions
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:         "superadmin",
		Capabilities: []composition.Capability{composition.CapabilityAPI},
		Requires:     []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddLocales(builder, &LocaleFiles)
	composition.AddNavItems(builder, NavItems...)

	composition.ProvideFunc(builder, persistence.NewPgAnalyticsQueryRepository)
	composition.ProvideFunc(builder, services.NewAnalyticsService)
	composition.ProvideFunc(builder, services.NewTenantService)
	composition.ProvideFunc(builder, services.NewTenantUsersService)

	composition.RemoveController(builder, "/")
	composition.ContributeControllersFunc(builder, func(userService *coreservices.UserService) []application.Controller {
		return []application.Controller{
			controllers.NewDashboardController(),
			controllers.NewTenantsController(userService),
		}
	})
	return nil
}
