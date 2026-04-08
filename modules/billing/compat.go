package billing

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

type Option = ComponentOption

func NewModule(opts ...Option) application.Module {
	return composition.Legacy(NewComponent(opts...), composition.CapabilityAPI)
}
