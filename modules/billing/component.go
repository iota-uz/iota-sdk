// Package billing provides this package.
package billing

import (
	"embed"

	billingdom "github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/providers"
	"github.com/iota-uz/iota-sdk/modules/billing/ports"
	"github.com/iota-uz/iota-sdk/modules/billing/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

type ComponentOption func(*component)

func WithStripeEventHooks(hooks ...ports.StripeEventHook) ComponentOption {
	return func(c *component) {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			c.stripeHooks = append(c.stripeHooks, hook)
		}
	}
}

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

func NewComponent(opts ...ComponentOption) composition.Component {
	component := &component{}
	for _, opt := range opts {
		if opt != nil {
			opt(component)
		}
	}
	return component
}

type component struct {
	stripeHooks []ports.StripeEventHook
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "billing",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	app := builder.Context().App
	conf := configuration.Use()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	logTransport := middleware.NewLogTransport(conf.Logger(), conf, true, true, "octo")
	clickProvider := providers.NewClickProvider(providers.ClickConfig{
		URL:            conf.Click.URL,
		ServiceID:      conf.Click.ServiceID,
		SecretKey:      conf.Click.SecretKey,
		MerchantID:     conf.Click.MerchantID,
		MerchantUserID: conf.Click.MerchantUserID,
	})
	paymeProvider := providers.NewPaymeProvider(providers.PaymeConfig{
		URL:        conf.Payme.URL,
		SecretKey:  conf.Payme.SecretKey,
		MerchantID: conf.Payme.MerchantID,
		User:       conf.Payme.User,
	})
	octoProvider := providers.NewOctoProvider(providers.OctoConfig{
		OctoShopID: conf.Octo.OctoShopID,
		OctoSecret: conf.Octo.OctoSecret,
		NotifyURL:  conf.Octo.NotifyURL,
	}, logTransport)
	stripeProvider := providers.NewStripeProvider(providers.StripeConfig{
		SecretKey: conf.Stripe.SecretKey,
	})

	billingService := services.NewBillingService(
		persistence.NewBillingRepository(),
		[]billingdom.Provider{clickProvider, paymeProvider, octoProvider, stripeProvider},
		app.EventPublisher(),
	)
	composition.Provide[*services.BillingService](builder, billingService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		basePath := "/billing"
		stripeHooks := append([]ports.StripeEventHook{}, c.stripeHooks...)
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			return []application.Controller{
				controllers.NewClickController(app, conf.Click, basePath+"/click"),
				controllers.NewPaymeController(app, conf.Payme, basePath+"/payme"),
				controllers.NewOctoController(app, conf.Octo, basePath+"/octo", logTransport),
				controllers.NewStripeController(app, conf.Stripe, basePath+"/stripe", stripeHooks...),
			}, nil
		})
	}

	return nil
}
