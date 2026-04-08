package crm

import (
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

type Module struct {
	composition.Component
}

func NewModule() *Module {
	return &Module{Component: NewComponent()}
}

func (m *Module) Name() string {
	return m.Descriptor().Name
}
