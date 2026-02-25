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
