// Package testkit provides this package.
package testkit

import (
	"github.com/iota-uz/iota-sdk/modules/testkit/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "testkit",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	if !builder.Context().HasCapability(composition.CapabilityAPI) {
		return nil
	}

	conf := configuration.Use()
	if !conf.EnableTestEndpoints {
		conf.Logger().Debug("Test endpoints disabled - testkit module not loading controllers")
		return nil
	}

	conf.Logger().Warn("Test endpoints enabled - this should only be used in test environments")
	composition.ContributeControllersFunc(builder, controllers.NewTestEndpointsController)

	return nil
}
