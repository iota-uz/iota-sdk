package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/require"
)

type lifecycleExecutor struct{ started, cancelled, release chan struct{} }

func (e *lifecycleExecutor) Execute(ctx context.Context, _ RunJobPayload) error {
	close(e.started)
	<-ctx.Done()
	close(e.cancelled)
	<-e.release
	return nil
}

// Falsely green if Execute finishes before lifecycle cancellation or Redis is never checked after joining.
func TestRunJobWorker_CancellationJoinsAcceptedExecutionAndAcknowledges(t *testing.T) {
	mr := miniredis.RunT(t)
	exec := &lifecycleExecutor{make(chan struct{}), make(chan struct{}), make(chan struct{})}
	worker, queue, _ := newTestRunJobWorker(t, mr, nil, func(cfg *RunJobWorkerConfig) { cfg.Executor = exec })
	payload := RunJobPayload{TenantID: uuid.New(), SessionID: uuid.New(), RunID: uuid.New(), RequestID: uuid.New()}
	_, _, err := queue.Enqueue(t.Context(), payload)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	defer close(exec.release)
	done := make(chan error, 1)
	go func() { done <- worker.Start(ctx) }()
	select {
	case <-exec.started:
	case <-time.After(time.Second):
		t.Fatal("execution did not start")
	}
	cancel()
	select {
	case <-exec.cancelled:
	case <-time.After(time.Second):
		t.Fatal("accepted executor did not receive cancellation")
	}
	select {
	case <-done:
		t.Fatal("worker returned before Execute finished")
	default:
	}
	exec.release <- struct{}{}
	select {
	case err = <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("worker did not join")
	}
	length, err := queue.client.XLen(t.Context(), queue.stream).Result()
	require.NoError(t, err)
	require.Zero(t, length)
}

type lifecycleAgent struct {
	stubAgentService
	started chan struct{}
}

func (a *lifecycleAgent) ProcessMessage(ctx context.Context, _ uuid.UUID, _ string, _ []domain.Attachment) (types.Generator[agents.ExecutorEvent], error) {
	close(a.started)
	return types.NewGenerator(ctx, func(ctx context.Context, _ func(agents.ExecutorEvent) bool) error { <-ctx.Done(); return ctx.Err() }), nil
}

// Falsely green if an existing ActiveRun bypasses the executor-owned context branch.
func TestRunExecutor_StandaloneGenerationRespectsOwnerCancellation(t *testing.T) {
	repo := newMockChatRepository()
	session := mustSession(t, withSessionTenantID(uuid.New()), withSessionUserID(1))
	require.NoError(t, repo.CreateSession(t.Context(), session))
	agent := &lifecycleAgent{started: make(chan struct{})}
	svc, err := NewChatService(repo, agent, nil, nil, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	job := RunJobPayload{TenantID: session.TenantID(), SessionID: session.ID(), RunID: uuid.New(), UserMessageID: uuid.New(), Content: "hello"}
	go func() { done <- svc.runExecutor.Execute(ctx, job) }()
	select {
	case <-agent.started:
	case <-time.After(time.Second):
		t.Fatal("generation did not start")
	}
	cancel()
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("generation detached from owner cancellation")
	}
}
