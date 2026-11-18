package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopMigrationManagerValidatesRollbackSteps(t *testing.T) {
	m := &noopMigrationManager{}
	require.Error(t, m.Rollback(context.Background(), 0))
	require.NoError(t, m.Rollback(context.Background(), 1))
}
