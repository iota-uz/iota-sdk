// Package crm provides this package.
package crm

import (
	"context"
	"embed"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	clientagg "github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/handlers"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twilio/twilio-go"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/crm-schema.sql
var MigrationFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "crm"}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddLocales(builder, &LocaleFiles)
	composition.AddNavItems(builder, NavItems...)
	composition.AddQuickLinks(builder, spotlight.NewQuickLink(ClientsLink.Name, ClientsLink.Href))
	composition.ContributeMigrations(builder, &MigrationFiles)

	composition.ContributeSpotlightProviders(builder, func(container *composition.Container) ([]spotlight.SearchProvider, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		return []spotlight.SearchProvider{newSpotlightProvider(pool)}, nil
	})

	composition.ProvideFunc(builder, corepersistence.NewPassportRepository)
	composition.ProvideFunc(builder, persistence.NewChatRepository)
	composition.ProvideFunc(builder, persistence.NewClientRepository)
	composition.ProvideFunc(builder, newCRMTwilioProvider)
	composition.ProvideFunc(builder, services.NewClientService)
	composition.ProvideFunc(builder, newCRMChatService)
	composition.ProvideFunc(builder, newCRMMessageTemplateService)

	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		app, err := composition.RequireApplication(container)
		if err != nil {
			return nil, err
		}
		chatService, err := composition.Resolve[*services.ChatService](container)
		if err != nil {
			return nil, err
		}
		tenantService, err := composition.Resolve[*coreservices.TenantService](container)
		if err != nil {
			return nil, err
		}
		hooks := []composition.Hook{
			{
				Name: "crm-client-handler",
				Start: func(context.Context) (composition.StopFn, error) {
					h := handlers.RegisterClientHandler(app, chatService, tenantService)
					return func(context.Context) error {
						if h != nil {
							h.Unregister()
						}
						return nil
					}, nil
				},
			},
			{
				Name: "crm-sms-handler",
				Start: func(context.Context) (composition.StopFn, error) {
					h := handlers.RegisterSMSHandlers(app, chatService)
					return func(context.Context) error {
						if h != nil {
							h.Unregister()
						}
						return nil
					}, nil
				},
			},
		}
		if botToken := configuration.Use().TelegramBotToken; botToken != "" {
			hooks = append(hooks, composition.Hook{
				Name: "crm-notification-handler",
				Start: func(context.Context) (composition.StopFn, error) {
					h := handlers.RegisterNotificationHandler(app, botToken)
					return func(context.Context) error {
						if h != nil {
							h.Unregister()
						}
						return nil
					}, nil
				},
			})
		}
		return hooks, nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(
			app application.Application,
			clientService *services.ClientService,
			chatService *services.ChatService,
			templateService *services.MessageTemplateService,
			userService *coreservices.UserService,
			tenantService *coreservices.TenantService,
			twilioProvider *cpassproviders.TwilioProvider,
		) []application.Controller {
			basePath := "/crm/clients"
			return []application.Controller{
				controllers.NewClientController(app, clientService, chatService, controllers.ClientControllerConfig{
					BasePath: basePath,
					Tabs: []controllers.TabDefinition{
						controllers.ProfileTab(basePath, clientService),
						controllers.ChatTab(basePath, clientService, chatService),
						controllers.ActionsTab(),
					},
				}),
				controllers.NewChatController(app, userService, clientService, chatService, templateService, tenantService, "/crm/chats"),
				controllers.NewMessageTemplateController(app, templateService, "/crm/instant-messages"),
				controllers.NewTwilioController(app, twilioProvider),
			}
		})
	}

	return nil
}

// newCRMTwilioProvider builds a Twilio provider with config from the app
// configuration. Used by ProvideFunc — pulls configuration directly so the
// constructor signature stays free of config types.
func newCRMTwilioProvider(
	clientRepo clientagg.Repository,
	chatRepo chat.Repository,
) *cpassproviders.TwilioProvider {
	cfg := configuration.Use()
	return cpassproviders.NewTwilioProvider(
		cpassproviders.Config{
			Params: twilio.ClientParams{
				Username: cfg.Twilio.AccountSID,
				Password: cfg.Twilio.AuthToken,
			},
			WebhookURL: cfg.Twilio.WebhookURL,
		},
		clientRepo,
		chatRepo,
	)
}

// newCRMChatService wraps services.NewChatService so the providers slice can
// be populated by the reflection injector. The injected twilioProvider is
// wrapped in the provider list inside the body.
func newCRMChatService(
	chatRepo chat.Repository,
	clientRepo clientagg.Repository,
	clientService *services.ClientService,
	twilioProvider *cpassproviders.TwilioProvider,
	bus eventbus.EventBus,
) *services.ChatService {
	return services.NewChatService(
		chatRepo,
		clientRepo,
		clientService,
		[]chat.Provider{twilioProvider},
		bus,
	)
}

// newCRMMessageTemplateService keeps message-template wiring local since the
// repository is built inline.
func newCRMMessageTemplateService(bus eventbus.EventBus) *services.MessageTemplateService {
	return services.NewMessageTemplateService(persistence.NewMessageTemplateRepository(), bus)
}
