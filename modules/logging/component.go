// Package logging provides this package.
package logging

import (
	"embed"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/health"
)

//go:embed presentation/locales/*.json
var localeFiles embed.FS

type ComponentOption func(*component)

// WithHistoryBasePath mounts the business history page at path instead of /admin/history.
func WithHistoryBasePath(path string) ComponentOption {
	return func(c *component) {
		c.historyBasePath = path
	}
}

func NewComponent(opts ...ComponentOption) composition.Component {
	c := &component{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type component struct {
	historyBasePath string
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "logging"}
}

func (c *component) LocaleFS() []*embed.FS {
	return []*embed.FS{&localeFiles}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.ProvideFunc(builder, func() (*history.Registry, error) {
		registry := history.NewRegistry()
		if err := registry.Register(history.LegacyActionLogDefinition(), history.ImportItemDefinition()); err != nil {
			return nil, err
		}
		return registry, nil
	})
	composition.ProvideFunc(builder, persistence.NewBusinessEventRepository)
	composition.ProvideFunc(builder, persistence.NewBusinessEventDeliveryRepository)
	composition.ProvideFunc(builder, history.NewConsumers)
	composition.ProvideFunc(builder, func(registry *history.Registry, repo businessevent.Repository, consumers *history.Consumers, deliveries businessevent.DeliveryRepository) history.Writer {
		return history.WithOutbox(history.NewWriter(registry, repo), consumers, deliveries)
	})
	composition.ProvideFunc(builder, history.NewDispatcher)
	composition.ContributeMiddleware(builder, func(*composition.Container) ([]mux.MiddlewareFunc, error) {
		return []mux.MiddlewareFunc{history.Middleware()}, nil
	})
	runsWorkers := builder.Context().HasCapability(composition.CapabilityAPI) || builder.Context().HasCapability(composition.CapabilityWorker)
	composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
		dispatcher, err := composition.Resolve[*history.Dispatcher](container)
		if err != nil {
			return nil, err
		}
		if capabilities, err := composition.Resolve[health.CapabilityRegistry](container); err == nil && capabilities != nil {
			capabilities.Register(dispatcher)
		}
		if !runsWorkers {
			return nil, nil
		}
		return []composition.Hook{{Name: "business-history-outbox", Start: dispatcher.Start}}, nil
	})
	composition.ProvideFunc(builder, history.NewService)
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllersFunc(builder, func(service *history.Service, registry *history.Registry) []application.Controller {
			return []application.Controller{controllers.NewHistoryController(c.historyBasePath, service, registry)}
		})
	}
	return nil
}
