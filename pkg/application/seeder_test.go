package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeeder_SeedRunsFuncsInOrder(t *testing.T) {
	seeder := NewSeeder()
	calls := make([]string, 0, 2)

	seeder.Register(
		func(ctx context.Context, deps *SeedDeps) error {
			calls = append(calls, "first")
			require.NotNil(t, deps)
			return nil
		},
		func(ctx context.Context, deps *SeedDeps) error {
			calls = append(calls, "second")
			require.NotNil(t, deps)
			return nil
		},
	)

	err := seeder.Seed(context.Background(), &SeedDeps{})
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, calls)
}

func TestSeeder_SeedStopsOnError(t *testing.T) {
	seeder := NewSeeder()
	expectedErr := errors.New("boom")
	calls := make([]string, 0, 2)

	seeder.Register(
		func(ctx context.Context, deps *SeedDeps) error {
			calls = append(calls, "first")
			return expectedErr
		},
		func(ctx context.Context, deps *SeedDeps) error {
			calls = append(calls, "second")
			return nil
		},
	)

	err := seeder.Seed(context.Background(), &SeedDeps{})
	require.ErrorIs(t, err, expectedErr)
	assert.Equal(t, []string{"first"}, calls)
}
