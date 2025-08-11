package projects

import (
	"embed"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/projects/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
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
		spotlight.NewQuickLink(nil, ProjectsItem.Name, ProjectsItem.Href),
		spotlight.NewQuickLink(nil, ProjectStagesItem.Name, ProjectStagesItem.Href),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Projects.List.New",
			"/projects/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"ProjectStages.List.New",
			"/project-stages/new",
		),
	)

	// Register permissions, locales, and migrations
	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.Migrations().RegisterSchema(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "projects"
}
