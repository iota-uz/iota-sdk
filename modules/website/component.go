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
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS


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
	composition.AddLocales(builder, &LocaleFiles)
	composition.AddNavItems(builder, NavItems...)
	composition.ProvideFunc(builder, persistence.NewAIChatConfigRepository)
	composition.ProvideFunc(builder, services.NewAIChatConfigService)
	composition.ProvideFunc(builder, newWebsiteChatService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.AddControllers(builder,
			controllers.NewAIChatController(controllers.AIChatControllerConfig{
				BasePath: "/website/ai-chat",
			}),
			controllers.NewAIChatAPIController(controllers.AIChatAPIControllerConfig{
				BasePath: "/api/website/ai-chat",
			}),
		)
	}

	return nil
}

// newWebsiteChatService is a thin adapter so ProvideFunc can resolve the
// constructor's dependencies by type. The real constructor takes a Config
// struct which the reflection injector cannot fill in.
func newWebsiteChatService(
	aiConfigRepo aichatconfig.Repository,
	userRepo coreuser.Repository,
	clientRepo clientagg.Repository,
	chatRepo chat.Repository,
) *services.WebsiteChatService {
	return services.NewWebsiteChatService(
		services.WebsiteChatServiceConfig{
			AIConfigRepo: aiConfigRepo,
			UserRepo:     userRepo,
			ClientRepo:   clientRepo,
			ChatRepo:     chatRepo,
			AIUserEmail:  internet.MustParseEmail("ai@llm.com"),
		},
	)
}
