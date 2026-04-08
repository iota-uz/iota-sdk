// Package projects provides this package.
package projects

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/projects-schema.sql
var migrationFiles embed.FS

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
	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})

	projectService := services.NewProjectService(
		persistence.NewProjectRepository(),
		app.EventPublisher(),
	)
	projectStageService := services.NewProjectStageService(
		persistence.NewProjectStageRepository(),
		app.EventPublisher(),
	)

	composition.Provide[*services.ProjectService](builder, projectService)
	composition.Provide[*services.ProjectStageService](builder, projectStageService)
	app.QuickLinks().Add(
		spotlight.NewQuickLink(ProjectsItem.Name, ProjectsItem.Href),
		spotlight.NewQuickLink(ProjectStagesItem.Name, ProjectStagesItem.Href),
		spotlight.NewQuickLink("Projects.List.New", "/projects/new"),
		spotlight.NewQuickLink("ProjectStages.List.New", "/project-stages/new"),
	)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			return []application.Controller{
				controllers.NewProjectController(app),
				controllers.NewProjectStageController(app),
			}, nil
		})
	}

	return nil
}
