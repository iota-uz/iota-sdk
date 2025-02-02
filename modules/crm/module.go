package crm

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/crm-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	clientRepo := persistence.NewClientRepository()
	app.RegisterServices(
		services.NewChatService(
			persistence.NewChatRepository(),
			clientRepo,
			app.EventPublisher(),
		),
		services.NewClientService(
			clientRepo,
			app.EventPublisher(),
		),
		services.NewMessageTemplateService(
			persistence.NewMessageTemplateRepository(),
			app.EventPublisher(),
		),
	)

	app.RegisterControllers(
		controllers.NewClientController(app, "/crm/clients"),
		controllers.NewChatController(app, "/crm/chats"),
		controllers.NewMessageTemplateController(app, "/crm/instant-messages"),
	)

	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterMigrationDirs(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "crm"
}
