package applet

import (
	"testing"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/go-i18n/v2/i18n"
	sdkapplet "github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

func BuildControllersForTest(
	t *testing.T,
	appletsToBuild []applets.Applet,
	pool *pgxpool.Pool,
	bundle *i18n.Bundle,
	host applets.HostServices,
	sessionConfig applets.SessionConfig,
	logger *logrus.Logger,
	metrics applets.MetricsRecorder,
	opts ...applets.BuilderOption,
) ([]Controller, error) {
	t.Helper()

	if logger == nil {
		logger = logrus.New()
	}
	if metrics == nil {
		metrics = sdkapplet.NewNoopMetricsRecorder()
	}

	result, err := NewAppletEngineBuilder().Build(BuildInput{
		Applets:       appletsToBuild,
		Pool:          pool,
		Bundle:        bundle,
		Host:          host,
		SessionConfig: sessionConfig,
		Logger:        logger,
		Metrics:       metrics,
		Options:       opts,
	})
	if err != nil {
		return nil, err
	}

	return result.Controllers, nil
}
