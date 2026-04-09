// Package projects provides this package.
package projects

import (
	"embed"

	project "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	projectstage "github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project_stage"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "projects",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	ctx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{
			spotlight.NewQuickLink(ProjectsItem.Name, ProjectsItem.Href),
			spotlight.NewQuickLink(ProjectStagesItem.Name, ProjectStagesItem.Href),
			spotlight.NewQuickLink("Projects.List.New", "/projects/new"),
			spotlight.NewQuickLink("ProjectStages.List.New", "/project-stages/new"),
		}, nil
	})

	projectRepo := composition.Use[project.Repository]()
	projectStageRepo := composition.Use[projectstage.Repository]()

	composition.Provide[project.Repository](builder, func() project.Repository {
		return persistence.NewProjectRepository()
	})
	composition.Provide[projectstage.Repository](builder, func() projectstage.Repository {
		return persistence.NewProjectStageRepository()
	})
	composition.Provide[*services.ProjectService](builder, func(container *composition.Container) (*services.ProjectService, error) {
		resolvedProjectRepo, err := projectRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewProjectService(resolvedProjectRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.ProjectStageService](builder, func(container *composition.Container) (*services.ProjectStageService, error) {
		resolvedProjectStageRepo, err := projectStageRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewProjectStageService(resolvedProjectStageRepo, ctx.EventPublisher()), nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			return []application.Controller{
				controllers.NewProjectController(app),
				controllers.NewProjectStageController(app),
			}, nil
		})
	}

	return nil
}
