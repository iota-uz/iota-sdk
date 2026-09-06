package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	api "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

// False-green guard: the worker only broadcasts; seeding its journal in the
// test would bypass the missing async-run wiring that broke HITL delivery.
func TestAsyncRunEvents_IdleWorkerThenTerminalReplay(t *testing.T) {
	t.Parallel()
	for _, operation := range []api.AsyncRunOperation{api.AsyncRunOperationContinuation, api.AsyncRunOperationQuestionSubmit, api.AsyncRunOperationQuestionReject} {
		t.Run(string(operation), func(t *testing.T) {
			t.Parallel()
			tailSvc, _, _, log := newTailTestService(t)
			repo := newMockChatRepository()
			session := mustSession(t, withSessionTenantID(uuid.New()), withSessionUserID(1))
			require.NoError(t, repo.CreateSession(t.Context(), session))
			svc, err := NewChatService(repo, &stubAgentService{}, nil, nil, nil)
			require.NoError(t, err)
			svc.eventLog = log
			svc.runState = tailSvc.runState
			ctx := composables.WithTenantID(t.Context(), session.TenantID())
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			finished := make(chan struct{})
			accepted, err := svc.startAsyncRun(ctx, session.ID(), operation, "", nil, func(_ context.Context, persistCtx context.Context, runID uuid.UUID, _ domain.Session, active *streaming.ActiveRun) {
				defer close(finished)
				<-release
				active.Broadcast(api.StreamChunk{Type: api.ChunkTypeContent, Content: "resumed answer"})
				active.Broadcast(streaming.TerminalChunk(nil, 1))
			})
			require.NoError(t, err)
			replay, err := log.Replay(ctx, session.TenantID(), accepted.RunID, "")
			require.NoError(t, err)
			require.Len(t, replay, 1, "the journal must exist before the worker produces output")
			require.Equal(t, "stream_started", replay[0].Type)
			// An idle tail must survive longer than Redis's BLOCK interval.
			tailCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			tail, err := log.Tail(tailCtx, session.TenantID(), accepted.RunID, replay[0].StreamID)
			require.NoError(t, err)
			select {
			case <-tail:
				t.Fatal("idle worker lost its event stream")
			case <-time.After(120 * time.Millisecond):
			}
			unblock()
			var types []string
			for event := range tail {
				types = append(types, event.Type)
			}
			require.Equal(t, []string{"content", "done"}, types)
			<-finished
			replay, err = log.Replay(ctx, session.TenantID(), accepted.RunID, replay[0].StreamID)
			require.NoError(t, err)
			require.Len(t, replay, 2, "reconnecting readers must recover the terminal event")
		})
	}
}

// False-green guard: no terminal event is inserted into the missing journal.
func TestTailRunEvents_MissingJournalIsNotSuccessfulEOF(t *testing.T) {
	t.Parallel()
	svc, _, store, _ := newTailTestService(t)
	tenant, session, run := uuid.New(), uuid.New(), uuid.New()
	seedTailRun(t, store, tenant, session, run)
	ctx, cancel := context.WithTimeout(composables.WithTenantID(t.Context(), tenant), time.Second)
	defer cancel()
	err := svc.TailRunEvents(ctx, session, run, "", func(api.RunEventDelivery) { t.Error("unexpected event") })
	require.Error(t, err)
	require.NoError(t, ctx.Err(), "must detect missing journal before the request times out")
}
