package crm

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/crm/handlers"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/twilio/twilio-go"
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
	conf := configuration.Use()
	twilioProvider := cpassproviders.NewTwilioProvider(
		twilio.ClientParams{
			Username: conf.Twilio.AccountSID,
			Password: conf.Twilio.AuthToken,
		},
		conf.Twilio.WebhookURL,
	)
	chatRepo := persistence.NewChatRepository()
	clientRepo := persistence.NewClientRepository()
	chatsService := services.NewChatService(
		chatRepo,
		clientRepo,
		twilioProvider,
		app.EventPublisher(),
	)
	app.RegisterServices(
		chatsService,
		services.NewClientService(
			clientRepo,
			chatsService,
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
		controllers.NewTwilioController(app, twilioProvider),
	)

	handlers.RegisterSMSHandlers(app)
	if botToken := conf.TelegramBotToken; botToken != "" {
		handlers.RegisterNotificationHandler(app, botToken)
	}

	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterSchemaFS(&migrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "crm"
}
