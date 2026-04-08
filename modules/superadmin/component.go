// Package superadmin provides this package.
package superadmin

import (
	"embed"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/superadmin/domain"
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
	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	composition.Provide[domain.AnalyticsQueryRepository](builder, func() domain.AnalyticsQueryRepository {
		return persistence.NewPgAnalyticsQueryRepository()
	})
	composition.Provide[coreuser.Repository](builder, func() coreuser.Repository {
		return corepersistence.NewUserRepository(corepersistence.NewUploadRepository())
	})
	composition.Provide[*services.AnalyticsService](builder, func(container *composition.Container) (*services.AnalyticsService, error) {
		repo, err := composition.Resolve[domain.AnalyticsQueryRepository](container)
		if err != nil {
			return nil, err
		}
		service := services.NewAnalyticsService(repo)
		builder.Context().App.RegisterServices(service)
		return service, nil
	})
	composition.Provide[*services.TenantService](builder, func(container *composition.Container) (*services.TenantService, error) {
		repo, err := composition.Resolve[domain.AnalyticsQueryRepository](container)
		if err != nil {
			return nil, err
		}
		service := services.NewTenantService(repo)
		builder.Context().App.RegisterServices(service)
		return service, nil
	})
	composition.Provide[*services.TenantUsersService](builder, func(container *composition.Container) (*services.TenantUsersService, error) {
		repo, err := composition.Resolve[coreuser.Repository](container)
		if err != nil {
			return nil, err
		}
		service := services.NewTenantUsersService(repo)
		builder.Context().App.RegisterServices(service)
		return service, nil
	})

	composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
		userService, err := composition.Resolve[*coreservices.UserService](container)
		if err != nil {
			return nil, err
		}
		return []application.Controller{
			controllers.NewDashboardController(builder.Context().App),
			controllers.NewTenantsController(builder.Context().App, userService),
		}, nil
	})

	return nil
}
