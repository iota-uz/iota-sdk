package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTailTestService stitches a minimal chatServiceImpl that exercises
// only the TailRunEvents path. Fields unused by the method are left nil
// deliberately — instantiating the full graph would drag the ITF harness
// in and we're unit-testing the tail flow here.
func newTailTestService(t *testing.T) (*chatServiceImpl, *miniredis.Miniredis, generationRunStore, *RedisRunEventLog) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)

	log, err := NewRedisRunEventLog(RedisRunEventLogConfig{
		Client:    client,
		BlockTime: 30 * time.Millisecond,
	})
	require.NoError(t, err)

	svc := &chatServiceImpl{
		runState: streaming.NewRunStateManager(store),
		eventLog: log,
	}
	return svc, mr, store, log
}

func seedTailRun(t *testing.T, store generationRunStore, tenantID, sessionID, runID uuid.UUID) {
	t.Helper()
	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		ID:        runID,
		SessionID: sessionID,
		TenantID:  tenantID,
		UserID:    1,
	})
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(context.Background(), run))
}

func TestChatService_TailRunEvents_ReplayThenTerminal(t *testing.T) {
	t.Parallel()

	svc, _, store, log := newTailTestService(t)
	tenantID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	seedTailRun(t, store, tenantID, sessionID, runID)

	push := func(typ string, body any) {
		payload, err := json.Marshal(body)
		require.NoError(t, err)
		_, err = log.Append(context.Background(), tenantID, runID, RunEvent{Type: typ, Payload: payload})
		require.NoError(t, err)
	}
	push("content", map[string]string{"text": "a"})
	push("content", map[string]string{"text": "b"})
	push("done", map[string]string{})

	ctx, cancel := context.WithTimeout(composables.WithTenantID(context.Background(), tenantID), 2*time.Second)
	defer cancel()

	var seen []string
	err := svc.TailRunEvents(ctx, sessionID, runID, "", func(evt bichatservices.RunEventDelivery) {
		seen = append(seen, evt.Type)
		assert.NotEmpty(t, evt.StreamID, "every delivered event must carry a stream id")
		assert.NotEmpty(t, evt.Payload, "payload must round-trip")
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"content", "content", "done"}, seen)
}

func TestChatService_TailRunEvents_HonoursLastEventID(t *testing.T) {
	t.Parallel()

	svc, _, store, log := newTailTestService(t)
	tenantID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	seedTailRun(t, store, tenantID, sessionID, runID)

	firstID, err := log.Append(context.Background(), tenantID, runID,
		RunEvent{Type: "content", Payload: json.RawMessage(`{"text":"one"}`)})
	require.NoError(t, err)
	_, err = log.Append(context.Background(), tenantID, runID,
		RunEvent{Type: "content", Payload: json.RawMessage(`{"text":"two"}`)})
	require.NoError(t, err)
	_, err = log.Append(context.Background(), tenantID, runID,
		RunEvent{Type: "done", Payload: json.RawMessage(`{}`)})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(composables.WithTenantID(context.Background(), tenantID), 2*time.Second)
	defer cancel()

	var seen []string
	err = svc.TailRunEvents(ctx, sessionID, runID, firstID, func(evt bichatservices.RunEventDelivery) {
		seen = append(seen, evt.Type)
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"content", "done"}, seen,
		"passing the first event's id must skip it and resume with the next (Last-Event-ID semantics)")
}

func TestChatService_TailRunEvents_MissingRunReturnsSentinel(t *testing.T) {
	t.Parallel()

	svc, _, _, _ := newTailTestService(t) //nolint:dogsled // harness returns 4 values; only svc needed here

	ctx := composables.WithTenantID(context.Background(), uuid.New())
	err := svc.TailRunEvents(ctx, uuid.New(), uuid.New(), "", func(evt bichatservices.RunEventDelivery) {
		t.Fatalf("onEvent must not be called for missing run: %+v", evt)
	})
	require.ErrorIs(t, err, bichatservices.ErrRunNotFoundOrFinished)
}

func TestChatService_TailRunEvents_SessionMismatchRejected(t *testing.T) {
	t.Parallel()

	svc, _, store, _ := newTailTestService(t)
	tenantID := uuid.New()
	sessionID := uuid.New()
	runID := uuid.New()
	seedTailRun(t, store, tenantID, sessionID, runID)

	ctx := composables.WithTenantID(context.Background(), tenantID)
	err := svc.TailRunEvents(ctx, uuid.New(), runID, "", func(evt bichatservices.RunEventDelivery) {
		t.Fatalf("must not deliver events across sessions: %+v", evt)
	})
	require.Error(t, err, "session id mismatch must fail the tail")
}

func TestChatService_TailRunEvents_NoLogReturnsSentinel(t *testing.T) {
	t.Parallel()

	svc := &chatServiceImpl{} // no runState, no eventLog
	err := svc.TailRunEvents(context.Background(), uuid.New(), uuid.New(), "", func(evt bichatservices.RunEventDelivery) {
		t.Fatalf("onEvent must not be called when event log is unconfigured")
	})
	require.ErrorIs(t, err, bichatservices.ErrRunEventLogUnavailable)
}

func TestChatService_TailRunEvents_ContextEndsBeforeTerminal(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"replay", "live_tail"} {
		for _, deadline := range []bool{false, true} {
			name := phase + "/cancelled"
			if deadline {
				name = phase + "/deadline"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				// False-green guard: seed a real journal but no terminal event;
				// otherwise a missing journal or normal completion masks the deadline.
				svc, _, store, log := newTailTestService(t)
				tenant, session, run := uuid.New(), uuid.New(), uuid.New()
				seedTailRun(t, store, tenant, session, run)
				for range 2 {
					_, err := log.Append(t.Context(), tenant, run, RunEvent{Type: "content", Payload: json.RawMessage(`{}`)})
					require.NoError(t, err)
				}
				parent := composables.WithTenantID(t.Context(), tenant)
				ctx, cancel := context.WithTimeout(parent, 200*time.Millisecond)
				defer cancel()
				seen := 0
				err := svc.TailRunEvents(ctx, session, run, "", func(bichatservices.RunEventDelivery) {
					seen++
					if phase == "replay" && seen == 1 {
						if !deadline {
							cancel()
						}
						<-ctx.Done()
					} else if phase == "live_tail" && seen == 2 && !deadline {
						cancel()
					}
				})
				if deadline {
					require.ErrorIs(t, err, bichatservices.ErrRunEventStreamInterrupted)
					require.ErrorIs(t, ctx.Err(), context.DeadlineExceeded)
				} else {
					require.NoError(t, err)
					require.ErrorIs(t, ctx.Err(), context.Canceled)
				}
				if phase == "replay" {
					assert.Equal(t, 1, seen)
				} else {
					assert.Equal(t, 2, seen)
				}
			})
		}
	}
}
