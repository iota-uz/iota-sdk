package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testDispatcher struct{}

func (d *testDispatcher) DispatchJob(context.Context, string, string, string, string, any) error {
	return nil
}

func TestNewRunner_RequiresDependencies(t *testing.T) {
	t.Parallel()

	runner, err := NewRunner(nil, &testDispatcher{}, logrus.New(), time.Second)
	require.Error(t, err)
	assert.Nil(t, runner)
	assert.Contains(t, err.Error(), "postgres pool is required")

	runner, err = NewRunner(nil, nil, logrus.New(), time.Second)
	require.Error(t, err)
	assert.Nil(t, runner)
}
