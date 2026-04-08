package composition

import "github.com/iota-uz/iota-sdk/pkg/application"

func Legacy(component Component, capabilities ...Capability) application.Module {
	return &legacyModule{
		component:    component,
		capabilities: append([]Capability(nil), capabilities...),
	}
}

type legacyModule struct {
	component         Component
	capabilities      []Capability
	app               application.Application
	container         *Container
	transportsApplied bool
}

func (m *legacyModule) Name() string {
	return normalizeDescriptor(m.component).Name
}

func (m *legacyModule) RegisterWiring(app application.Application) error {
	if m.container == nil || m.app != app {
		engine := NewEngine()
		if err := engine.Register(m.component); err != nil {
			return err
		}
		container, err := engine.Compile(BuildContext{App: app}, m.capabilities...)
		if err != nil {
			return err
		}
		if err := Apply(app, container, ApplyOptions{}); err != nil {
			return err
		}
		m.app = app
		m.container = container
		m.transportsApplied = false
	}
	return nil
}

func (m *legacyModule) RegisterTransports(app application.Application) error {
	if err := m.RegisterWiring(app); err != nil {
		return err
	}
	if m.transportsApplied {
		return nil
	}
	if err := Apply(app, m.container, ApplyOptions{IncludeControllers: true}); err != nil {
		return err
	}
	m.transportsApplied = true
	return nil
}
