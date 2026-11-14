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
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/pagination"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twilioconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
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

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&LocaleFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddNavNodes(builder, CRMLink)
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
	history.ContributeDefinitions(builder, services.HistoryDefinitions()...)
	composition.ProvideFunc(builder, services.NewClientService)
	composition.ProvideFunc(builder, newCRMChatService)
	composition.ProvideFunc(builder, services.NewMessageTemplateService)
	composition.ProvideFunc(builder, handlers.NewClientHandler)

	composition.ContributeEventHandlerFunc(builder, func(h *handlers.ClientHandler) any { return h.OnCreated })

	// The Telegram gate runs at Build time so it reads the composition
	// BuildContext directly rather than going through the auto-provider
	// path (providers are not yet resolvable here).
	if src := builder.Context().Source(); src != nil {
		var appCfg appconfig.Config
		if err := src.Unmarshal("app", &appCfg); err == nil && appCfg.TelegramBotToken != "" {
			notification, err := handlers.NewNotificationHandler(appCfg.TelegramBotToken)
			if err != nil {
				return err
			}
			composition.ContributeEventHandler(builder, notification.OnNewMessage)
		}
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			return crmControllers(container)
		})
	}

	return nil
}

// newCRMTwilioProvider builds a Twilio provider from the composition-injected
// configuration. Parameters are resolved by the reflection injector.
func newCRMTwilioProvider(
	clientRepo clientagg.Repository,
	chatRepo chat.Repository,
	cfg *twilioconfig.Config,
) *cpassproviders.TwilioProvider {
	return cpassproviders.NewTwilioProvider(
		cpassproviders.Config{
			Params: twilio.ClientParams{
				Username: cfg.AccountSID,
				Password: cfg.AuthToken,
			},
			WebhookURL: cfg.WebhookURL,
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

// crmControllers builds the CRM controllers. The client forms dispatch the
// published client commands through the application's native.Dispatcher (the
// extension runtime's host when it is enabled, the Go defaults otherwise).
func crmControllers(container *composition.Container) ([]application.Controller, error) {
	app, err := composition.Resolve[application.Application](container)
	if err != nil {
		return nil, err
	}
	clientService, err := composition.Resolve[*services.ClientService](container)
	if err != nil {
		return nil, err
	}
	chatService, err := composition.Resolve[*services.ChatService](container)
	if err != nil {
		return nil, err
	}
	templateService, err := composition.Resolve[*services.MessageTemplateService](container)
	if err != nil {
		return nil, err
	}
	userService, err := composition.Resolve[*coreservices.UserService](container)
	if err != nil {
		return nil, err
	}
	tenantService, err := composition.Resolve[*coreservices.TenantService](container)
	if err != nil {
		return nil, err
	}
	twilioProvider, err := composition.Resolve[*cpassproviders.TwilioProvider](container)
	if err != nil {
		return nil, err
	}
	logger, err := composition.Resolve[*logrus.Logger](container)
	if err != nil {
		return nil, err
	}
	paginationCfg, err := composition.Resolve[*pagination.Config](container)
	if err != nil {
		return nil, err
	}
	dispatcher, err := native.ResolveDispatcher(container, services.Contracts())
	if err != nil {
		return nil, err
	}
	basePath := ClientsPath
	return []application.Controller{
		controllers.NewClientController(app, clientService, chatService, logger, controllers.ClientControllerConfig{
			Dispatcher: dispatcher,
			BasePath:   basePath,
			Tabs: []controllers.TabDefinition{
				controllers.ProfileTab(basePath, clientService),
				controllers.ChatTab(basePath, clientService, chatService),
				controllers.ActionsTab(),
			},
		}),
		controllers.NewChatController(app, userService, clientService, chatService, templateService, tenantService, dispatcher, "/crm/chats", logger, paginationCfg),
		controllers.NewMessageTemplateController(templateService, "/crm/instant-messages"),
		controllers.NewTwilioController(app, twilioProvider),
	}, nil
}
