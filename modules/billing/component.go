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
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/paymentsconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
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
			paymentsCfg *paymentsconfig.Config,
			httpCfg *headers.Config,
		) []application.Controller {
			return newBillingControllers(billingSvc, paymentsCfg, httpCfg, stripeHooks)
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
	paymentsCfg *paymentsconfig.Config,
	httpCfg *headers.Config,
	logger *logrus.Logger,
) *services.BillingService {
	logTransport := middleware.NewLogTransport(logger, httpCfg, true, true, "octo")
	clickProvider := providers.NewClickProvider(providers.ClickConfig{
		URL:            paymentsCfg.Click.URL,
		ServiceID:      paymentsCfg.Click.ServiceID,
		SecretKey:      paymentsCfg.Click.SecretKey,
		MerchantID:     paymentsCfg.Click.MerchantID,
		MerchantUserID: paymentsCfg.Click.MerchantUserID,
	})
	paymeProvider := providers.NewPaymeProvider(providers.PaymeConfig{
		URL:        paymentsCfg.Payme.URL,
		SecretKey:  paymentsCfg.Payme.SecretKey,
		MerchantID: paymentsCfg.Payme.MerchantID,
		User:       paymentsCfg.Payme.User,
	})
	octoProvider := providers.NewOctoProvider(providers.OctoConfig{
		OctoShopID: paymentsCfg.Octo.ShopID,
		OctoSecret: paymentsCfg.Octo.Secret,
		NotifyURL:  paymentsCfg.Octo.NotifyURL,
	}, logTransport)
	stripeProvider := providers.NewStripeProvider(providers.StripeConfig{
		SecretKey: paymentsCfg.Stripe.SecretKey,
	})
	return services.NewBillingService(
		repo,
		[]billingdom.Provider{clickProvider, paymeProvider, octoProvider, stripeProvider},
		bus,
	)
}

func newBillingControllers(
	billingSvc *services.BillingService,
	paymentsCfg *paymentsconfig.Config,
	httpCfg *headers.Config,
	stripeHooks []ports.StripeEventHook,
) []application.Controller {
	basePath := "/billing"
	logTransport := middleware.NewLogTransport(logrus.StandardLogger(), httpCfg, true, true, "octo")
	return []application.Controller{
		controllers.NewClickController(billingSvc, paymentsCfg.Click, basePath+"/click"),
		controllers.NewPaymeController(billingSvc, paymentsCfg.Payme, basePath+"/payme"),
		controllers.NewOctoController(billingSvc, paymentsCfg.Octo, basePath+"/octo", logTransport),
		controllers.NewStripeController(billingSvc, paymentsCfg.Stripe, basePath+"/stripe", stripeHooks...),
	}
}
