// Package crm provides this package.
package crm

import (
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
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/twilio/twilio-go"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

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
	composition.ProvideFunc(builder, persistence.NewMessageTemplateRepository)
	composition.ProvideFunc(builder, newCRMTwilioProvider)
	composition.ProvideFunc(builder, services.NewClientService)
	composition.ProvideFunc(builder, newCRMChatService)
	composition.ProvideFunc(builder, services.NewMessageTemplateService)
	composition.ProvideFunc(builder, handlers.NewClientHandler)

	composition.ContributeEventHandlerFunc(builder, func(h *handlers.ClientHandler) any { return h.OnCreated })

	// The Telegram gate runs at Build time so it reads the composition
	// BuildContext directly rather than going through the auto-provider
	// path (providers are not yet resolvable here).
	if cfg := builder.Context().Config(); cfg != nil && cfg.TelegramBotToken != "" {
		notification, err := handlers.NewNotificationHandler(cfg.TelegramBotToken)
		if err != nil {
			return err
		}
		composition.ContributeEventHandler(builder, notification.OnNewMessage)
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(
			app application.Application,
			clientService *services.ClientService,
			chatService *services.ChatService,
			templateService *services.MessageTemplateService,
			userService *coreservices.UserService,
			tenantService *coreservices.TenantService,
			twilioProvider *cpassproviders.TwilioProvider,
			logger *logrus.Logger,
			httpCfg *httpconfig.Config,
		) []application.Controller {
			basePath := "/crm/clients"
			return []application.Controller{
				controllers.NewClientController(app, clientService, chatService, logger, controllers.ClientControllerConfig{
					BasePath: basePath,
					Tabs: []controllers.TabDefinition{
						controllers.ProfileTab(basePath, clientService),
						controllers.ChatTab(basePath, clientService, chatService),
						controllers.ActionsTab(),
					},
				}),
				controllers.NewChatController(app, userService, clientService, chatService, templateService, tenantService, "/crm/chats", logger, httpCfg),
				controllers.NewMessageTemplateController(templateService, "/crm/instant-messages"),
				controllers.NewTwilioController(app, twilioProvider),
			}
		})
	}

	return nil
}

// newCRMTwilioProvider builds a Twilio provider from the composition-injected
// configuration. Parameters are resolved by the reflection injector.
func newCRMTwilioProvider(
	clientRepo clientagg.Repository,
	chatRepo chat.Repository,
	cfg *configuration.Configuration,
) *cpassproviders.TwilioProvider {
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
