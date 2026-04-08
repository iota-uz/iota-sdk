package application

import (
	"github.com/iota-uz/applets"
	compositionapplet "github.com/iota-uz/iota-sdk/pkg/composition/applet"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

func (app *application) buildAppletControllersAndRuntime(
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, []RuntimeRegistration, error) {
	const op serrors.Op = "application.buildAppletControllersAndRuntime"
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
		return nil, nil, serrors.E(op, err)
	}
	controllers := make([]Controller, 0, len(result.Controllers))
	for _, controller := range result.Controllers {
		controllers = append(controllers, controller.(Controller))
	}
	registrations := make([]RuntimeRegistration, 0, len(result.RuntimeRegistrations))
	for _, registration := range result.RuntimeRegistrations {
		registrations = append(registrations, RuntimeRegistration{
			Component: newAppletRuntimeComponent(registration.Manager, app.DB(), logger, registration.HasPostgresJobs),
			Tags:      []RuntimeTag{RuntimeTagWorker},
		})
	}
	app.appletRuntime = result.RuntimeManager
	return controllers, registrations, nil
}
