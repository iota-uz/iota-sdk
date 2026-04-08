package bichat

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

func NewModule() *Module {
	return NewModuleWithConfig(nil)
}

func NewModuleWithConfig(cfg *ModuleConfig) *Module {
	component := NewComponent(cfg).(*component)
	return &Module{
		Component: component,
		component: component,
	}
}

type Module struct {
	composition.Component
	component *component
}

func (m *Module) Name() string {
	return m.Descriptor().Name
}

func (m *Module) Shutdown(ctx context.Context) error {
	if m == nil || m.component == nil {
		return nil
	}
	return m.component.Shutdown(ctx)
}
