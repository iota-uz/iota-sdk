// Package website provides this package.
package website

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
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
	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	userRepo := corePersistence.NewUserRepository(corePersistence.NewUploadRepository())
	chatRepo := crmPersistence.NewChatRepository()
	passportRepo := corePersistence.NewPassportRepository()
	clientRepo := crmPersistence.NewClientRepository(passportRepo)
	aiconfigRepo := persistence.NewAIChatConfigRepository()

	aiChatConfigService := services.NewAIChatConfigService(aiconfigRepo)
	websiteChatService := services.NewWebsiteChatService(
		services.WebsiteChatServiceConfig{
			AIConfigRepo: aiconfigRepo,
			UserRepo:     userRepo,
			ClientRepo:   clientRepo,
			ChatRepo:     chatRepo,
			AIUserEmail:  internet.MustParseEmail("ai@llm.com"),
		},
	)
	composition.Provide[*services.AIChatConfigService](builder, aiChatConfigService)
	composition.Provide[*services.WebsiteChatService](builder, websiteChatService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
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
