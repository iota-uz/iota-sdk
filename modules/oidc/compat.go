package oidc

import (
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

type Module struct {
	composition.Component
}

func NewModule(opts *ModuleOptions) *Module {
	return &Module{Component: NewComponent(opts)}
}

func (m *Module) Name() string {
	return m.Descriptor().Name
}
