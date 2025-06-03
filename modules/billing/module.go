package billing

import (
	"embed"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/providers"
	"github.com/iota-uz/iota-sdk/modules/billing/permissions"
	"github.com/iota-uz/iota-sdk/modules/billing/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type Module struct {
}

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/billing-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

func (m *Module) Register(app application.Application) error {
	conf := configuration.Use()

	clickProvider := providers.NewClickProvider(
		providers.ClickConfig{
			URL:            conf.Click.URL,
			ServiceID:      conf.Click.ServiceID,
			SecretKey:      conf.Click.SecretKey,
			MerchantID:     conf.Click.MerchantID,
			MerchantUserID: conf.Click.MerchantUserID,
		},
	)

	paymeProvider := providers.NewPaymeProvider(
		providers.PaymeConfig{
			URL:        conf.Payme.URL,
			SecretKey:  conf.Payme.SecretKey,
			MerchantID: conf.Payme.MerchantID,
			User:       conf.Payme.User,
		},
	)

	octoProvider := providers.NewOctoProvider(
		providers.OctoConfig{
			OctoShopID: conf.Octo.OctoShopID,
			OctoSecret: conf.Octo.OctoSecret,
			NotifyURL:  conf.Octo.NotifyUrl,
		},
		middleware.NewLogTransport(
			conf.Logger(),
			true,
			true,
		),
	)

	stripeProvider := providers.NewStripeProvider(
		providers.StripeConfig{
			SecretKey: conf.Stripe.SecretKey,
		},
	)

	billingProviders := []billing.Provider{
		clickProvider,
		paymeProvider,
		octoProvider,
		stripeProvider,
	}

	billingRepo := persistence.NewBillingRepository()

	billingService := services.NewBillingService(
		billingRepo,
		billingProviders,
		app.EventPublisher(),
	)

	app.RegisterServices(
		billingService,
	)

	basePath := "/billing"
	app.RegisterControllers(
		controllers.NewClickController(
			app,
			conf.Click,
			basePath+"/click",
		),
		controllers.NewPaymeController(
			app,
			conf.Payme,
			basePath+"/payme",
		),
		controllers.NewOctoController(
			app,
			conf.Octo,
			basePath+"/octo",
		),
		controllers.NewStripeController(
			app,
			conf.Stripe,
			basePath+"/stripe",
		),
	)

	app.RBAC().Register(permissions.Permissions...)
	app.RegisterLocaleFiles(&localeFiles)
	app.Migrations().RegisterSchema(&migrationFiles)

	return nil
}

func (m *Module) Name() string {
	return "billing"
}
