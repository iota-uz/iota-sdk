package bootstrap

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeShutdownCancelsLifecycleAndCleansUpOnce(t *testing.T) {
	var calls []string
	rt, cleanup, err := NewRuntime(context.Background(),
		WithLoggerFactory(func(context.Context, any) (*logrus.Logger, func() error, error) {
			return logrus.New(), func() error { calls = append(calls, "logger"); return nil }, nil
		}),
		WithPoolFactory(func(context.Context, any, *logrus.Logger) (*pgxpool.Pool, func() error, error) {
			return nil, func() error { calls = append(calls, "pool"); return nil }, nil
		}),
		WithBundleFactory(func(context.Context, any) (*i18n.Bundle, error) { return &i18n.Bundle{}, nil }),
		// No application: this test covers only the factory cleanups and the lifecycle context.
		WithApplicationFactory(func(context.Context, *Runtime) (application.Application, error) { return nil, nil }), //nolint:nilnil // intentional: runtime without an application

	)
	require.NoError(t, err)

	lifecycle := rt.Context()
	require.NoError(t, lifecycle.Err())

	require.NoError(t, rt.Shutdown(context.Background()))
	require.ErrorIs(t, lifecycle.Err(), context.Canceled)
	assert.Equal(t, []string{"pool", "logger"}, calls)

	// The deferred cleanup from NewRuntime must not tear down twice.
	require.NoError(t, cleanup())
	assert.Equal(t, []string{"pool", "logger"}, calls)
}
