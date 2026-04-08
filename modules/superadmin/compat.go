package superadmin

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

func NewModule(opts *ModuleOptions) application.Module {
	return composition.Legacy(NewComponent(opts), composition.CapabilityAPI)
}
