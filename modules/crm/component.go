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
		passportRepo, err := composition.Resolve[passport.Repository](container)
		if err != nil {
			return nil, err
		}
		return persistence.NewClientRepository(passportRepo), nil
	})
	composition.Provide[*cpassproviders.TwilioProvider](builder, func(container *composition.Container) (*cpassproviders.TwilioProvider, error) {
		clientRepo, err := composition.Resolve[clientagg.Repository](container)
		if err != nil {
			return nil, err
		}
		chatRepo, err := composition.Resolve[chat.Repository](container)
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
			clientRepo,
			chatRepo,
		), nil
	})
	composition.Provide[*services.ClientService](builder, func(container *composition.Container) (*services.ClientService, error) {
		clientRepo, err := composition.Resolve[clientagg.Repository](container)
		if err != nil {
			return nil, err
		}
		service := services.NewClientService(clientRepo, app.EventPublisher())
		app.RegisterServices(service)
		return service, nil
	})
	composition.Provide[*services.ChatService](builder, func(container *composition.Container) (*services.ChatService, error) {
		chatRepo, err := composition.Resolve[chat.Repository](container)
		if err != nil {
			return nil, err
		}
		clientRepo, err := composition.Resolve[clientagg.Repository](container)
		if err != nil {
			return nil, err
		}
		clientService, err := composition.Resolve[*services.ClientService](container)
		if err != nil {
			return nil, err
		}
		twilioProvider, err := composition.Resolve[*cpassproviders.TwilioProvider](container)
		if err != nil {
			return nil, err
		}
		service := services.NewChatService(
			chatRepo,
			clientRepo,
			clientService,
			[]chat.Provider{twilioProvider},
			app.EventPublisher(),
		)
		app.RegisterServices(service)
		return service, nil
	})
	composition.Provide[*services.MessageTemplateService](builder, func() *services.MessageTemplateService {
		service := services.NewMessageTemplateService(
			persistence.NewMessageTemplateRepository(),
			app.EventPublisher(),
		)
		app.RegisterServices(service)
		return service
	})
	composition.Provide[*handlers.ClientHandler](builder, func(container *composition.Container) (*handlers.ClientHandler, error) {
		chatService, err := composition.Resolve[*services.ChatService](container)
		if err != nil {
			return nil, err
		}
		tenantService, err := composition.Resolve[*coreservices.TenantService](container)
		if err != nil {
			return nil, err
		}
		return handlers.RegisterClientHandler(app, chatService, tenantService), nil
	})
	composition.Provide[*handlers.SMSHandler](builder, func(container *composition.Container) (*handlers.SMSHandler, error) {
		chatService, err := composition.Resolve[*services.ChatService](container)
		if err != nil {
			return nil, err
		}
		return handlers.RegisterSMSHandlers(app, chatService), nil
	})
	if botToken := config.TelegramBotToken; botToken != "" {
		composition.Provide[*handlers.NotificationHandler](builder, func() *handlers.NotificationHandler {
			return handlers.RegisterNotificationHandler(app, botToken)
		})
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
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
			}, nil
		})
	}

	return nil
}
