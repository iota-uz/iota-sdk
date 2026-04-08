package bichat

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

func NewModule() application.Module {
	return NewModuleWithConfig(nil)
}

func NewModuleWithConfig(cfg *ModuleConfig) *Module {
	component := NewComponent(cfg).(*component)
	return &Module{
		component: component,
		legacy: composition.Legacy(
			component,
			composition.CapabilityAPI,
			composition.CapabilityWorker,
		),
	}
}

type Module struct {
	component *component
	legacy    application.Module
}

func (m *Module) RegisterWiring(app application.Application) error {
	return m.legacy.RegisterWiring(app)
}

func (m *Module) RegisterTransports(app application.Application) error {
	return m.legacy.RegisterTransports(app)
}

func (m *Module) Name() string {
	return m.legacy.Name()
}

func (m *Module) Shutdown(ctx context.Context) error {
	if m == nil || m.component == nil {
		return nil
	}
	return m.component.Shutdown(ctx)
}
