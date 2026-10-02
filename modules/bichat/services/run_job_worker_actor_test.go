package services_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type actorLookup struct{ actor user.User }

func (l actorLookup) GetByID(context.Context, uint) (user.User, error) { return l.actor, nil }

type checkpointExecutor struct{ saved chan error }

func (e checkpointExecutor) Execute(ctx context.Context, job services.RunJobPayload) error {
	checkpoint := agents.NewCheckpoint("worker-thread", "test", []types.Message{}, agents.WithTenantID(job.TenantID), agents.WithSessionID(job.SessionID), agents.WithInterruptType(agents.ToolAskUserQuestion))
	_, err := persistence.NewPostgresCheckpointer().Save(ctx, checkpoint)
	e.saved <- err
	return err
}

// Falsely green if a constructor assertion replaces a real queued job and PostgreSQL checkpoint write.
func TestRunJobWorkerHydratesActorForPostgresCheckpoint(t *testing.T) {
	dsn := os.Getenv("TESTENV_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("TESTENV_POSTGRES_DSN is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := "testenv_actor_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, dropErr := admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{name}.Sanitize())
		require.NoError(t, dropErr)
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, err = pool.Exec(ctx, `CREATE SCHEMA bichat; CREATE TABLE bichat.checkpoints (id text PRIMARY KEY,thread_id text,tenant_id uuid,user_id bigint,agent_name text,messages jsonb,pending_tools jsonb,interrupt_type text,interrupt_data jsonb,session_id uuid,previous_response_id text,created_at timestamptz)`)
	require.NoError(t, err)
	actor := user.New("Actor", "Worker", internet.MustParseEmail("actor@example.com"), user.UILanguageEN, user.WithID(42), user.WithTenantID(uuid.New()))
	for _, mode := range []string{"actor", "foreign_tenant", "missing_actor"} {
		t.Run(mode, func(t *testing.T) {
			mr := miniredis.RunT(t)
			queue, queueErr := services.NewRedisRunJobQueue(services.RedisRunJobQueueConfig{RedisURL: mr.Addr()})
			require.NoError(t, queueErr)
			defer queue.Close()
			saved := make(chan error, 1)
			failed := make(chan struct{}, 1)
			worker, workerErr := services.NewRunJobWorker(services.RunJobWorkerConfig{Queue: queue, Executor: checkpointExecutor{saved}, Pool: pool, Users: actorLookup{actor}, ReadBlock: time.Millisecond, PollInterval: time.Millisecond, MaxRetries: 1, OnJobTerminalFailure: func(context.Context, services.RunJobPayload, error) { failed <- struct{}{} }})
			require.NoError(t, workerErr)
			job := services.RunJobPayload{TenantID: actor.TenantID(), UserID: 42, SessionID: uuid.New(), RunID: uuid.New(), RequestID: uuid.New(), UserMessageID: uuid.New()}
			if mode == "foreign_tenant" {
				job.TenantID = uuid.New()
			}
			if mode == "missing_actor" {
				job.UserID = 0
			}
			_, _, queueErr = queue.Enqueue(ctx, job)
			require.NoError(t, queueErr)
			workerCtx, workerCancel := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() { done <- worker.Start(workerCtx) }()
			defer func() {
				workerCancel()
				select {
				case <-done:
				case <-ctx.Done():
					t.Fatal("worker did not stop")
				}
			}()
			if mode != "actor" {
				select {
				case <-failed:
				case <-ctx.Done():
					t.Fatal("foreign actor was not rejected")
				}
				select {
				case <-saved:
					t.Fatal("foreign actor reached executor")
				default:
				}
				return
			}
			select {
			case saveErr := <-saved:
				require.NoError(t, saveErr)
			case <-ctx.Done():
				t.Fatal("checkpoint was not saved")
			}
			var actorID int64
			var tenant uuid.UUID
			require.NoError(t, pool.QueryRow(ctx, "SELECT user_id,tenant_id FROM bichat.checkpoints WHERE session_id=$1", job.SessionID).Scan(&actorID, &tenant))
			require.Equal(t, job.UserID, actorID)
			require.Equal(t, job.TenantID, tenant)
		})
	}
}
