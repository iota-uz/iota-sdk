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

type Module struct{}

func (m *Module) Register(app application.Application) error {
	// Register migrations
	app.Migrations().RegisterSchema(&MigrationFiles)

	// Register locale files
	app.RegisterLocaleFiles(&LocaleFiles)

	// Register repository
	chatRepo := persistence.NewPostgresChatRepository()

	// TODO(Phase 1): Replace with real implementations when Agent Framework is complete
	// Register stub services
	chatService := services.NewChatServiceStub()

	// Register controllers
	app.RegisterControllers(
		controllers.NewChatController(app, chatService, chatRepo),
		controllers.NewStreamController(app, chatService),
	)

	// Register quick links
	app.QuickLinks().Add(
		spotlight.NewQuickLink(BiChatLink.Icon, BiChatLink.Name, BiChatLink.Href),
	)

	return nil
}

func (m *Module) Name() string {
	return "bichat"
}
