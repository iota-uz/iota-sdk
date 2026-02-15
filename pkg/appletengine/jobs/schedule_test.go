package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNextRun(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 2, 12, 10, 15, 0, 0, time.UTC)
	next, err := NextRun("*/5 * * * *", base)
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 2, 12, 10, 20, 0, 0, time.UTC), next)
}

func TestNextRun_InvalidCron(t *testing.T) {
	t.Parallel()

	_, err := NextRun("not-a-cron", time.Now().UTC())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse cron expression")
}
