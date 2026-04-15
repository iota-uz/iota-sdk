package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewGenerationRun_RequiresStreamingSpec(t *testing.T) {
	t.Parallel()

	_, err := domain.NewGenerationRun(domain.GenerationRunSpec{})
	require.Error(t, err)

	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusStreaming, run.Status())
}

func TestGenerationRun_Transitions(t *testing.T) {
	t.Parallel()

	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
	})
	require.NoError(t, err)

	updated, err := run.UpdateSnapshot("partial", map[string]any{"k": "v"}, time.Now())
	require.NoError(t, err)
	assert.Equal(t, "partial", updated.PartialContent())

	completed, err := updated.Complete(time.Now())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusCompleted, completed.Status())

	_, err = completed.UpdateSnapshot("x", nil, time.Now())
	require.Error(t, err)

	_, err = completed.Cancel(time.Now())
	require.Error(t, err)
}

func TestGenerationRun_Fail_OnlyFromStreaming(t *testing.T) {
	t.Parallel()

	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
	})
	require.NoError(t, err)

	failed, err := run.Fail(time.Now())
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusFailed, failed.Status())

	_, err = failed.Fail(time.Now())
	require.Error(t, err, "double-failing must be rejected")

	_, err = failed.Cancel(time.Now())
	require.Error(t, err, "cancel on failed run must be rejected")
}

func TestGenerationRun_RequestCancel_StaysStreaming(t *testing.T) {
	t.Parallel()

	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
	})
	require.NoError(t, err)
	require.False(t, run.CancelRequested(), "new run must start without a cancel request")

	requested, err := run.RequestCancel(time.Now())
	require.NoError(t, err)
	assert.True(t, requested.CancelRequested())
	assert.Equal(t, domain.GenerationRunStatusStreaming, requested.Status(),
		"RequestCancel must not change status; the worker drives Cancel")

	// Request again: idempotent, no error.
	again, err := requested.RequestCancel(time.Now())
	require.NoError(t, err)
	assert.True(t, again.CancelRequested())
}

func TestGenerationRun_Heartbeat_RefreshesTimestamp(t *testing.T) {
	t.Parallel()

	start := time.Now().Add(-time.Minute)
	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
		StartedAt: start,
	})
	require.NoError(t, err)
	assert.True(t, run.LastHeartbeatAt().IsZero(), "never-heartbeated runs must report a zero LastHeartbeatAt")

	beat := time.Now()
	beat1, err := run.Heartbeat(beat)
	require.NoError(t, err)
	assert.Equal(t, beat.UTC().Truncate(0), beat1.LastHeartbeatAt().UTC().Truncate(0))

	// Heartbeat on a completed run is rejected.
	completed, err := beat1.Complete(time.Now())
	require.NoError(t, err)
	_, err = completed.Heartbeat(time.Now())
	require.Error(t, err)
}

func TestRehydrateGenerationRun_AcceptsFailedStatus(t *testing.T) {
	t.Parallel()

	run, err := domain.RehydrateGenerationRun(domain.GenerationRunSpec{
		ID:        uuid.New(),
		SessionID: uuid.New(),
		TenantID:  uuid.New(),
		UserID:    17,
		Status:    domain.GenerationRunStatusFailed,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusFailed, run.Status())
}
