package billing

import (
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

type Option = ComponentOption

type Module struct {
	composition.Component
}

func NewModule(opts ...Option) *Module {
	return &Module{Component: NewComponent(opts...)}
}

func (m *Module) Name() string {
	return m.Descriptor().Name
}
