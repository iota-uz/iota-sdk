package testing

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/subscription"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockEngine_ReservationLifecycleMatchesEngineSemantics(t *testing.T) {
	t.Parallel()

	mock := NewMockEngine()
	mock.SetLimit("drivers", 3)

	subject := subscription.Subject{
		Scope: subscription.ScopeTenant,
		ID:    uuid.New(),
	}
	quota := subscription.QuotaKey{
		Resource: "drivers",
		Window:   subscription.WindowNone,
	}

	reservation, err := mock.Reserve(context.Background(), subject, quota, 2, "token-1")
	require.NoError(t, err)

	decisionAfterReserve, err := mock.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 2, decisionAfterReserve.Current)
	assert.Equal(t, 1, decisionAfterReserve.Remaining)

	require.NoError(t, mock.Commit(context.Background(), reservation.ID))
	decisionAfterCommit, err := mock.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 2, decisionAfterCommit.Current)

	require.NoError(t, mock.Commit(context.Background(), reservation.ID))
	decisionAfterSecondCommit, err := mock.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 2, decisionAfterSecondCommit.Current)

	require.NoError(t, mock.Release(context.Background(), reservation.ID))
	decisionAfterRelease, err := mock.EvaluateLimit(context.Background(), subject, quota)
	require.NoError(t, err)
	assert.Equal(t, 0, decisionAfterRelease.Current)
}
