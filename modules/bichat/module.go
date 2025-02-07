package bichat

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/bichat-schema.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RegisterMigrationDirs(&MigrationFiles)
	app.RegisterLocaleFiles(&LocaleFiles)
	app.RegisterServices(
		services.NewEmbeddingService(app),
	)
	app.RegisterServices(
		services.NewDialogueService(persistence.NewDialogueRepository(), app),
	)
	app.RegisterControllers(
		controllers.NewBiChatController(app),
	)
	app.Spotlight().Register(
		spotlight.NewItem(nil, BiChatLink.Name, BiChatLink.Href),
	)
	return nil
}

func (m *Module) Name() string {
	return "bichat"
}
