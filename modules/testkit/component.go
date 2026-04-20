// Package testkit provides this package.
package testkit

import (
	"github.com/iota-uz/iota-sdk/modules/testkit/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twofactorconfig"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
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

	composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
		appCfg, err := composition.Resolve[*appconfig.Config](container)
		if err != nil {
			return nil, err
		}
		logger, err := composition.Resolve[*logrus.Logger](container)
		if err != nil {
			return nil, err
		}
		if !appCfg.EnableTestEndpoints {
			logger.Debug("Test endpoints disabled - testkit module not loading controllers")
			return nil, nil
		}
		logger.Warn("Test endpoints enabled - this should only be used in test environments")
		pool, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		twofactorCfg, err := composition.Resolve[*twofactorconfig.Config](container)
		if err != nil {
			return nil, err
		}
		ctrl := controllers.NewTestEndpointsController(pool, appCfg, twofactorCfg)
		return []application.Controller{ctrl}, nil
	})

	return nil
}
