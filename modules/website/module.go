package website

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/website-schema.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	userRepo := corePersistence.NewUserRepository(
		corePersistence.NewUploadRepository(),
	)
	chatRepo := crmPersistence.NewChatRepository()
	passportRepo := corePersistence.NewPassportRepository()
	clientRepo := crmPersistence.NewClientRepository(
		passportRepo,
	)
	aiconfigRepo := persistence.NewAIChatConfigRepository()
	app.RegisterServices(
		services.NewAIChatConfigService(aiconfigRepo),
		services.NewWebsiteChatService(
			services.WebsiteChatServiceConfig{
				AIConfigRepo: aiconfigRepo,
				UserRepo:     userRepo,
				ClientRepo:   clientRepo,
				ChatRepo:     chatRepo,
				AIUserEmail:  internet.MustParseEmail("ai@llm.com"),
			},
		),
	)
	app.RegisterControllers(
		controllers.NewAIChatController(controllers.AIChatControllerConfig{
			BasePath: "/website/ai-chat",
			App:      app,
		}),
	)
	app.RegisterLocaleFiles(&LocaleFiles)
	app.Migrations().RegisterSchema(&MigrationFiles)
	app.RegisterHashFsAssets(assets.HashFS)
	return nil
}

func (m *Module) Name() string {
	return "website"
}
