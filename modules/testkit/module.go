package testkit

import (
	"github.com/iota-uz/iota-sdk/modules/testkit/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type Module struct{}

func NewModule() application.Module {
	return &Module{}
}

func (m *Module) Register(app application.Application) error {
	conf := configuration.Use()

	// Only register test endpoints if explicitly enabled
	if !conf.EnableTestEndpoints {
		conf.Logger().Debug("Test endpoints disabled - testkit module not loading controllers")
		return nil
	}

	conf.Logger().Warn("Test endpoints enabled - this should only be used in test environments")

	// Register test endpoints controller
	app.RegisterControllers(
		controllers.NewTestEndpointsController(app),
	)

	return nil
}

func (m *Module) Name() string {
	return "testkit"
}

func (m *Module) Key() string {
	return "testkit"
}
