// Package website provides this package.
package website

import (
	"embed"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	clientagg "github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/website-schema.sql
var MigrationFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "website",
		Requires: []string{"core", "crm"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})

	userRepo := composition.Use[coreuser.Repository]()
	chatRepo := composition.Use[chat.Repository]()
	clientRepo := composition.Use[clientagg.Repository]()
	aiConfigRepo := composition.Use[aichatconfig.Repository]()

	composition.Provide[aichatconfig.Repository](builder, func() aichatconfig.Repository {
		return persistence.NewAIChatConfigRepository()
	})
	composition.Provide[*services.AIChatConfigService](builder, func(container *composition.Container) (*services.AIChatConfigService, error) {
		resolvedAIConfigRepo, err := aiConfigRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewAIChatConfigService(resolvedAIConfigRepo), nil
	})
	composition.Provide[*services.WebsiteChatService](builder, func(container *composition.Container) (*services.WebsiteChatService, error) {
		resolvedAIConfigRepo, err := aiConfigRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUserRepo, err := userRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedClientRepo, err := clientRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedChatRepo, err := chatRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewWebsiteChatService(
			services.WebsiteChatServiceConfig{
				AIConfigRepo: resolvedAIConfigRepo,
				UserRepo:     resolvedUserRepo,
				ClientRepo:   resolvedClientRepo,
				ChatRepo:     resolvedChatRepo,
				AIUserEmail:  internet.MustParseEmail("ai@llm.com"),
			},
		), nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			return []application.Controller{
				controllers.NewAIChatController(controllers.AIChatControllerConfig{
					BasePath: "/website/ai-chat",
					App:      app,
				}),
				controllers.NewAIChatAPIController(controllers.AIChatAPIControllerConfig{
					BasePath: "/api/website/ai-chat",
					App:      app,
				}),
			}, nil
		})
	}

	return nil
}
