package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCrossTenantIsolation_ReplayReturnsEmpty verifies that tenant B cannot
// read events appended for tenant A's run, even when tenant B knows the run
// ID. The stream key embeds the tenant ID so a guessed run ID is useless
// across tenant boundaries.
func TestCrossTenantIsolation_ReplayReturnsEmpty(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	runA := uuid.New()

	// Append an event as tenant A.
	body, err := json.Marshal(map[string]string{"text": "secret"})
	require.NoError(t, err)
	_, err = log.Append(context.Background(), tenantA, runA, RunEvent{
		Type:    "content",
		Payload: body,
	})
	require.NoError(t, err)

	// Tenant B tries to replay using tenant A's run ID — must see nothing.
	events, err := log.Replay(context.Background(), tenantB, runA, RunEventStreamStart)
	require.NoError(t, err)
	assert.Empty(t, events, "tenant B must not see tenant A's events")
}

// TestCrossTenantIsolation_TailReturnsNoEvents verifies that tailing with
// tenant B's context against tenant A's run ID yields no events within a
// short deadline. The stream key for tenant B is different from tenant A's
// so XREAD returns nothing.
func TestCrossTenantIsolation_TailReturnsNoEvents(t *testing.T) {
	t.Parallel()

	log, _ := newTestRunEventLog(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	runA := uuid.New()

	// Append an event as tenant A.
	body, err := json.Marshal(map[string]string{"text": "secret"})
	require.NoError(t, err)
	_, err = log.Append(context.Background(), tenantA, runA, RunEvent{
		Type:    "content",
		Payload: body,
	})
	require.NoError(t, err)

	// Tenant B tails the same run ID — must receive nothing within the deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch, err := log.Tail(ctx, tenantB, runA, RunEventStreamStart)
	require.NoError(t, err)

	select {
	case evt, ok := <-ch:
		if ok {
			t.Errorf("tenant B received an event from tenant A's run: type=%q", evt.Type)
		}
		// Channel closed without yielding data — acceptable (ctx deadline).
	case <-ctx.Done():
		// Context expired before any event arrived — correct isolation.
	}
}
