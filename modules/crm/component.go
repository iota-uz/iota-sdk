// Package crm provides this package.
package crm

import (
	"embed"

	passport "github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
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
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
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
	app := builder.Context().App
	config := configuration.Use()
	passportRepo := composition.Use[passport.Repository]()
	chatRepo := composition.Use[chat.Repository]()
	clientRepo := composition.Use[clientagg.Repository]()
	twilioProvider := composition.Use[*cpassproviders.TwilioProvider]()
	clientService := composition.Use[*services.ClientService]()
	chatService := composition.Use[*services.ChatService]()
	templateService := composition.Use[*services.MessageTemplateService]()
	userService := composition.Use[*coreservices.UserService]()
	tenantService := composition.Use[*coreservices.TenantService]()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeSpotlightProviders(builder, func(*composition.Container) ([]spotlight.SearchProvider, error) {
		return []spotlight.SearchProvider{newSpotlightProvider(app.DB())}, nil
	})

	app.QuickLinks().Add(spotlight.NewQuickLink(ClientsLink.Name, ClientsLink.Href))

	composition.Provide[passport.Repository](builder, func() passport.Repository {
		return corepersistence.NewPassportRepository()
	})
	composition.Provide[chat.Repository](builder, func() chat.Repository {
		return persistence.NewChatRepository()
	})
	composition.Provide[clientagg.Repository](builder, func(container *composition.Container) (clientagg.Repository, error) {
		resolvedPassportRepo, err := passportRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return persistence.NewClientRepository(resolvedPassportRepo), nil
	})
	composition.Provide[*cpassproviders.TwilioProvider](builder, func(container *composition.Container) (*cpassproviders.TwilioProvider, error) {
		resolvedClientRepo, err := clientRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedChatRepo, err := chatRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return cpassproviders.NewTwilioProvider(
			cpassproviders.Config{
				Params: twilio.ClientParams{
					Username: config.Twilio.AccountSID,
					Password: config.Twilio.AuthToken,
				},
				WebhookURL: config.Twilio.WebhookURL,
			},
			resolvedClientRepo,
			resolvedChatRepo,
		), nil
	})
	composition.Provide[*services.ClientService](builder, func(container *composition.Container) (*services.ClientService, error) {
		resolvedClientRepo, err := clientRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewClientService(resolvedClientRepo, app.EventPublisher()), nil
	})
	composition.Provide[*services.ChatService](builder, func(container *composition.Container) (*services.ChatService, error) {
		resolvedChatRepo, err := chatRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedClientRepo, err := clientRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedClientService, err := clientService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedTwilioProvider, err := twilioProvider.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewChatService(
			resolvedChatRepo,
			resolvedClientRepo,
			resolvedClientService,
			[]chat.Provider{resolvedTwilioProvider},
			app.EventPublisher(),
		), nil
	})
	composition.Provide[*services.MessageTemplateService](builder, func() *services.MessageTemplateService {
		return services.NewMessageTemplateService(
			persistence.NewMessageTemplateRepository(),
			app.EventPublisher(),
		)
	})
	composition.Provide[*handlers.ClientHandler](builder, func(container *composition.Container) (*handlers.ClientHandler, error) {
		resolvedChatService, err := chatService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedTenantService, err := tenantService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return handlers.RegisterClientHandler(app, resolvedChatService, resolvedTenantService), nil
	})
	composition.Provide[*handlers.SMSHandler](builder, func(container *composition.Container) (*handlers.SMSHandler, error) {
		resolvedChatService, err := chatService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return handlers.RegisterSMSHandlers(app, resolvedChatService), nil
	})
	if botToken := config.TelegramBotToken; botToken != "" {
		composition.Provide[*handlers.NotificationHandler](builder, func() *handlers.NotificationHandler {
			return handlers.RegisterNotificationHandler(app, botToken)
		})
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			resolvedClientService, err := clientService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedChatService, err := chatService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedTemplateService, err := templateService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedUserService, err := userService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedTenantService, err := tenantService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedTwilioProvider, err := twilioProvider.Resolve(container)
			if err != nil {
				return nil, err
			}
			basePath := "/crm/clients"
			return []application.Controller{
				controllers.NewClientController(app, resolvedClientService, resolvedChatService, controllers.ClientControllerConfig{
					BasePath: basePath,
					Tabs: []controllers.TabDefinition{
						controllers.ProfileTab(basePath, resolvedClientService),
						controllers.ChatTab(basePath, resolvedClientService, resolvedChatService),
						controllers.ActionsTab(),
					},
				}),
				controllers.NewChatController(app, resolvedUserService, resolvedClientService, resolvedChatService, resolvedTemplateService, resolvedTenantService, "/crm/chats"),
				controllers.NewMessageTemplateController(app, resolvedTemplateService, "/crm/instant-messages"),
				controllers.NewTwilioController(app, resolvedTwilioProvider),
			}, nil
		})
	}

	return nil
}
