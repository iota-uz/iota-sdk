package crm

import (
	"embed"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/handlers"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/permissions"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/twilio/twilio-go"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/crm-schema.sql
var MigrationFiles embed.FS

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

	passportRepo := corepersistence.NewPassportRepository()
	chatRepo := persistence.NewChatRepository()
	clientRepo := persistence.NewClientRepository(passportRepo)
	chatsService := services.NewChatService(
		chatRepo,
		clientRepo,
		[]chat.Provider{twilioProvider},
		app.EventPublisher(),
	)
	app.RegisterServices(
		chatsService,
		services.NewClientService(
			clientRepo,
			app.EventPublisher(),
		),
		services.NewMessageTemplateService(
			persistence.NewMessageTemplateRepository(),
			app.EventPublisher(),
		),
	)

	app.QuickLinks().Add(
		spotlight.NewQuickLink(ClientsLink.Icon, ClientsLink.Name, ClientsLink.Href),
	)
	app.Spotlight().Register(&ClientDataSource{})

	// Configure client controller with explicit tabs
	basePath := "/crm/clients"
	app.RegisterControllers(
		controllers.NewClientController(app, controllers.ClientControllerConfig{
			BasePath: basePath,
			Tabs: []controllers.TabDefinition{
				controllers.ProfileTab(basePath),
				controllers.ChatTab(basePath),
				controllers.ActionsTab(),
			},
		}),
		controllers.NewChatController(app, "/crm/chats"),
		controllers.NewMessageTemplateController(app, "/crm/instant-messages"),
		// TODO: uncomment when Twilio is available
		// controllers.NewTwilioController(app, twilioProvider),
	)

	handlers.RegisterClientHandler(app)
	handlers.RegisterSMSHandlers(app)
	if botToken := conf.TelegramBotToken; botToken != "" {
		handlers.RegisterNotificationHandler(app, botToken)
	}

	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&LocaleFiles)
	app.Migrations().RegisterSchema(&MigrationFiles)
	return nil
}

func (m *Module) Name() string {
	return "crm"
}
