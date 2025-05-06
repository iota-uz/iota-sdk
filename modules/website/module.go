package website

import (
	"embed"

	corePersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	crmPersistence "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/website/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/website/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/sashabaranov/go-openai"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

////go:embed infrastructure/persistence/schema/warehouse-schema.sql
//var migrationFiles embed.FS

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
			openai.NewClient(configuration.Use().OpenAIKey),
			aiconfigRepo,
			userRepo,
			clientRepo,
			chatRepo,
		),
	)
	app.RegisterControllers(
		controllers.NewAIChatController(app),
	)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterHashFsAssets(assets.HashFS)
	return nil
}

func (m *Module) Name() string {
	return "website"
}
