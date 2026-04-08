package application

import (
	"github.com/iota-uz/applets"
	compositionapplet "github.com/iota-uz/iota-sdk/pkg/composition/applet"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

func (app *application) buildAppletControllers(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, error) {
	const op serrors.Op = "application.buildAppletControllers"
	builder := compositionapplet.NewAppletEngineBuilder()
	result, err := builder.Build(compositionapplet.BuildInput{
		Applets:       app.AppletRegistry().All(),
		Pool:          app.DB(),
		Bundle:        app.Bundle(),
		Host:          host,
		SessionConfig: sessionConfig,
		Logger:        logger,
		Metrics:       metrics,
		Options:       opts,
	})
	if err != nil {
		return nil, serrors.E(op, err)
	}
	controllers := make([]Controller, 0, len(result.Controllers))
	for _, controller := range result.Controllers {
		controllers = append(controllers, controller.(Controller))
	}
	return controllers, nil
}
