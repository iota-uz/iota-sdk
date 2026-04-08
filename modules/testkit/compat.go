package testkit

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

func NewModule() application.Module {
	return composition.Legacy(NewComponent(), composition.CapabilityAPI)
}
