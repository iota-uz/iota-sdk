package projects

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/projects-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	_ = migrationFiles

	// Register services
	projectService := services.NewProjectService(
		persistence.NewProjectRepository(),
		app.EventPublisher(),
	)
	projectStageService := services.NewProjectStageService(
		persistence.NewProjectStageRepository(),
		app.EventPublisher(),
	)

	app.RegisterServices(
		projectService,
		projectStageService,
	)

	// Register controllers
	app.RegisterControllers(
		controllers.NewProjectController(app),
		controllers.NewProjectStageController(app),
	)

	// Register quick links
	app.QuickLinks().Add(
		spotlight.NewQuickLink(ProjectsItem.Name, ProjectsItem.Href),
		spotlight.NewQuickLink(ProjectStagesItem.Name, ProjectStagesItem.Href),
		spotlight.NewQuickLink("Projects.List.New",
			"/projects/new",
		),
		spotlight.NewQuickLink("ProjectStages.List.New",
			"/project-stages/new",
		),
	)

	// Register locales and migrations
	// Note: Permissions are now registered via defaults.AllPermissions()
	app.RegisterLocaleFiles(&localeFiles)
	return nil
}

func (m *Module) Name() string {
	return "projects"
}
