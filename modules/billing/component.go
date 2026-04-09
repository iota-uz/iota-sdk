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
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
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
	composition.AddLocales(builder, &LocaleFiles)

	composition.ProvideFuncAs[billingdom.Repository](builder, persistence.NewBillingRepository)
	composition.ProvideFunc(builder, newBillingService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		stripeHooks := append([]ports.StripeEventHook{}, c.stripeHooks...)
		composition.ContributeControllersFunc(builder, func(
			billingSvc *services.BillingService,
			conf *configuration.Configuration,
		) []application.Controller {
			return newBillingControllers(billingSvc, conf, stripeHooks)
		})
	}

	return nil
}

// newBillingService is the wide-dependency constructor that the reflection
// injector can wire directly. Configuration is injected through the
// composition container so callers can substitute a test/override config
// without touching process-global state.
func newBillingService(
	repo billingdom.Repository,
	bus eventbus.EventBus,
	conf *configuration.Configuration,
) *services.BillingService {
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
	return services.NewBillingService(
		repo,
		[]billingdom.Provider{clickProvider, paymeProvider, octoProvider, stripeProvider},
		bus,
	)
}

func newBillingControllers(
	billingSvc *services.BillingService,
	conf *configuration.Configuration,
	stripeHooks []ports.StripeEventHook,
) []application.Controller {
	basePath := "/billing"
	logTransport := middleware.NewLogTransport(conf.Logger(), conf, true, true, "octo")
	return []application.Controller{
		controllers.NewClickController(billingSvc, conf.Click, basePath+"/click"),
		controllers.NewPaymeController(billingSvc, conf.Payme, basePath+"/payme"),
		controllers.NewOctoController(billingSvc, conf.Octo, basePath+"/octo", logTransport),
		controllers.NewStripeController(billingSvc, conf.Stripe, basePath+"/stripe", stripeHooks...),
	}
}
