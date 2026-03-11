package application

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeeder_SeedRunsFuncsInOrder(t *testing.T) {
	seeder := NewSeeder()
	calls := make([]string, 0, 2)
	deps := &SeedDeps{Logger: logrus.New()}

	seeder.Register(
		Seed(func(ctx context.Context, logger logrus.FieldLogger) error {
			calls = append(calls, "first")
			require.NotNil(t, logger)
			return nil
		}),
		Seed(func(ctx context.Context, logger logrus.FieldLogger) error {
			calls = append(calls, "second")
			require.NotNil(t, logger)
			return nil
		}),
	)

	err := seeder.Seed(context.Background(), deps)
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, calls)
}

func TestSeeder_SeedStopsOnError(t *testing.T) {
	seeder := NewSeeder()
	expectedErr := errors.New("boom")
	calls := make([]string, 0, 2)
	deps := &SeedDeps{Logger: logrus.New()}

	seeder.Register(
		Seed(func(ctx context.Context, logger logrus.FieldLogger) error {
			calls = append(calls, "first")
			return expectedErr
		}),
		Seed(func(ctx context.Context, logger logrus.FieldLogger) error {
			calls = append(calls, "second")
			return nil
		}),
	)

	err := seeder.Seed(context.Background(), deps)
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []string{"first"}, calls)
}

func TestSeedDeps_InvokeUsesRegisteredValues(t *testing.T) {
	type sampleService struct {
		Name string
	}

	logger := logrus.New()
	service := &sampleService{Name: "seed"}
	deps := &SeedDeps{Logger: logger}
	deps.RegisterValues(service)

	var resolvedName string
	err := deps.Invoke(context.Background(), func(ctx context.Context, svc *sampleService, injectedLogger logrus.FieldLogger) error {
		resolvedName = svc.Name
		require.Same(t, logger, injectedLogger)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "seed", resolvedName)
}

func TestSeedDeps_InvokeFailsOnMissingDependency(t *testing.T) {
	deps := &SeedDeps{}

	err := deps.Invoke(context.Background(), func(ctx context.Context, logger logrus.FieldLogger) error {
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider found")
}

func TestSeedPanicsOnInvalidSignature(t *testing.T) {
	assert.PanicsWithError(t,
		"seed function must accept context.Context as the first argument",
		func() {
			Seed(func(logger logrus.FieldLogger) error {
				return nil
			})
		},
	)
}
