// Package services provides this package.
package services

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// streamPersistenceTimeout bounds each detached persistence transaction that
// outlives the client request: stream finalize (assistant message + session,
// and best-effort artifacts), HITL resume/reject, history clear/compact, and
// run-state writes. Deep-mode runs produce large payloads, so this is generous:
// a too-tight budget previously discarded fully-generated answers when
// persistence overran it (see #2998).
const streamPersistenceTimeout = 30 * time.Second
const titleGenerationFallbackTimeout = 15 * time.Second
const streamSnapshotThrottle = 2 * time.Second
const remoteResumePollInterval = time.Second
const runStateFinalizationAttempts = 3
const runStateFinalizationRetryDelay = 25 * time.Millisecond

// chatServiceImpl is the production implementation of ChatService.
// It orchestrates chat sessions, messages, and agent execution.
type chatServiceImpl struct {
	chatRepo      domain.ChatRepository
	sessionAccess domain.SessionAccessRepository
	agentService  bichatservices.AgentService
	model         agents.Model
	titleService  TitleService
	titleQueue    TitleJobQueue
	runState      *streamingsvc.RunStateManager
	// eventLog mirrors every broadcast chunk into a durable per-run Redis
	// stream so a client that reconnects (new tab, new device, network
	// blip) can tail or replay from a cursor instead of reconstructing
	// state from in-memory buffers. nil when Redis is unconfigured, in
	// which case only the in-memory broadcaster is used and cross-process
	// resume is unavailable.
	eventLog RunEventLog
	// activeRunIndex maintains the per-tenant sidebar view of running
	// sessions. nil when Redis is unconfigured — in that mode sidebar
	// dots degrade to polling via /stream/status, but the core
	// streaming path still works.
	activeRunIndex ActiveRunIndex
	// runJobQueue is used only for its ClaimRequest side in the inline
	// path (request_id idempotency). In run-workers mode it is also the
	// enqueue surface: SendMessageStream XADDs the job instead of running
	// the generation in-process. nil when Redis is unconfigured → dedupe
	// silently degrades to the pre-existing "two concurrent sends on same
	// session → second fails ErrActiveRunExists" behaviour and enqueue
	// mode is unavailable.
	runJobQueue *RedisRunJobQueue
	// runSessionQueue is the per-session FIFO for messages sent while a
	// run is active. nil when Redis is unconfigured (queueing disabled).
	runSessionQueue *RedisRunSessionQueue
	// runWorkersEnabled switches SendMessageStream to enqueue mode: jobs
	// are handed to Redis run workers instead of executing inline. It
	// requires Redis; when the queue/event log are missing the service
	// falls back to the inline path automatically.
	runWorkersEnabled bool
	// runExecutor executes a generation turn from a payload. The inline
	// path spawns it in-process; the Redis run worker calls it for
	// consumed jobs.
	runExecutor RunExecutor
	// closeSharedRedis is set when a shared *redis.Client was created by
	// newConfiguredRedisComponents. Must be called exactly once at shutdown
	// (via CloseSharedRedis). nil when Redis is unconfigured.
	closeSharedRedis   func() error
	streamCancelMu     sync.Mutex
	activeStreamCancel map[uuid.UUID]context.CancelFunc
	runRegistry        *streamingsvc.RunRegistry
	logger             *logrus.Logger
	// langfuseBaseURL is the Langfuse host URL for building trace links in debug
	// traces. Set via WithLangfuseBaseURL; empty string disables trace URL generation.
	langfuseBaseURL string
}

// NewChatService creates a production implementation of ChatService.
// Returns an error when REDIS_URL is set but any Redis component fails to
// initialise — this prevents a broken Redis config from silently degrading
// to the in-memory fallback while operators believe Redis is active.
//
// Example:
//
//	service, err := NewChatService(chatRepo, agentService, model, titleService, titleQueue)
func NewChatService(
	chatRepo domain.ChatRepository,
	agentService bichatservices.AgentService,
	model agents.Model,
	titleService TitleService,
	titleQueue TitleJobQueue,
) (*chatServiceImpl, error) {
	const op serrors.Op = "NewChatService"
	runStore := newConfiguredGenerationRunStore()
	accessRepo := chatRepo.(domain.SessionAccessRepository)
	// Use a single shared Redis connection for all Redis-backed components so
	// the process dials only one connection to Redis. Falls back to nil/noop
	// when REDIS_URL is unset (dev/CI without Redis).
	// closeSharedRedis is the single owner of the shared *redis.Client; each
	// component's Close() is a no-op because the client was supplied
	// externally. Call CloseSharedRedis() exactly once at shutdown.
	eventLog, activeRunIndex, runJobQueue, runSessionQueue, closeSharedRedis, err := newConfiguredRedisComponents()
	if err != nil {
		return nil, serrors.E(op, err)
	}
	core := &chatServiceImpl{
		chatRepo:           chatRepo,
		sessionAccess:      accessRepo,
		agentService:       agentService,
		model:              model,
		titleService:       titleService,
		titleQueue:         normalizeTitleJobQueue(titleQueue),
		runState:           streamingsvc.NewRunStateManager(runStore),
		eventLog:           eventLog,
		activeRunIndex:     activeRunIndex,
		runJobQueue:        runJobQueue,
		runSessionQueue:    runSessionQueue,
		closeSharedRedis:   closeSharedRedis,
		activeStreamCancel: make(map[uuid.UUID]context.CancelFunc),
		runRegistry:        streamingsvc.NewRunRegistry(),
	}
	core.runExecutor = newChatRunExecutor(core)
	return core, nil
}

// WithLogger injects a logrus.Logger into the service. Call immediately after
// NewChatService; safe before concurrent use. No-op when logger is nil.
func (s *chatServiceImpl) WithLogger(logger *logrus.Logger) *chatServiceImpl {
	if logger != nil {
		s.logger = logger
	}
	return s
}

// WithLangfuseBaseURL stores the Langfuse host URL for trace link generation.
// The URL may be the BaseURL or Host field from LangfuseConfig; callers should
// prefer BaseURL, falling back to Host when BaseURL is empty.
func (s *chatServiceImpl) WithLangfuseBaseURL(rawURL string) *chatServiceImpl {
	s.langfuseBaseURL = strings.TrimSpace(rawURL)
	return s
}

// WithRunWorkersEnabled switches SendMessageStream to enqueue mode: send
// requests hand the generation job to the Redis run queue and become pure
// event-log cursor readers instead of executing the agent in-process.
// Call immediately after NewChatService; safe before concurrent use. The
// mode additionally requires Redis (queue + event log) at send time —
// without it the service automatically falls back to the inline path.
func (s *chatServiceImpl) WithRunWorkersEnabled(enabled bool) *chatServiceImpl {
	s.runWorkersEnabled = enabled
	return s
}

// RunExecutor exposes the turn executor so the run worker (and tests) can
// drive generation from a RunJobPayload.
func (s *chatServiceImpl) RunExecutor() RunExecutor {
	return s.runExecutor
}

// FailStalledRun drives a run to the failed terminal state after its job was
// lost to an infrastructure failure the worker could not retry out of
// (executor absent, retries exhausted). Mirrors RunReaper.failStaleRun:
// Redis-side terminal transition + sanitized error event + sidebar publish.
// The PostgreSQL row stays streaming on purpose — RestartRun reconciles it
// on the next send for the session, exactly like reaped runs.
func (s *chatServiceImpl) FailStalledRun(ctx context.Context, job RunJobPayload, cause error) {
	const op serrors.Op = "chatServiceImpl.FailStalledRun"

	runStateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), streamPersistenceTimeout)
	defer cancel()
	if err := s.finalizeRunState(
		runStateCtx,
		job.TenantID,
		job.SessionID,
		job.RunID,
		string(domain.GenerationRunStatusFailed),
		s.runState.FailRunState,
	); err != nil {
		s.log().WithError(serrors.E(op, err)).WithField("run_id", job.RunID.String()).
			Warn("bichat: failed to persist stalled-run state")
	}

	if s.eventLog != nil {
		errPayload := streamingsvc.TerminalChunk(serrors.E(op, cause), 0)
		if eventType, body, encodeErr := encodeRunEventFromChunk(errPayload); encodeErr == nil {
			if _, appendErr := s.eventLog.Append(runStateCtx, job.TenantID, job.RunID, RunEvent{
				Type:    eventType,
				Payload: body,
			}); appendErr == nil {
				_ = s.eventLog.DropAfterTerminal(runStateCtx, job.TenantID, job.RunID, 5*time.Minute)
			}
		}
	}

	s.publishTerminalStatus(runStateCtx, job.TenantID, job.SessionID, job.RunID, string(domain.GenerationRunStatusFailed))
}

// logEntry returns a logrus.Entry for the given fields, or nil when no logger
// is configured. Callers do: if e := s.logEntry(); e != nil { e.WithField(...).Warn(...) }
func (s *chatServiceImpl) logEntry() *logrus.Entry {
	if s.logger == nil {
		return nil
	}
	return logrus.NewEntry(s.logger)
}

// log returns the service logger, falling back to the standard logger when none
// was injected. Used by critical-path error logging that must never be silenced.
func (s *chatServiceImpl) log() *logrus.Logger {
	if s.logger != nil {
		return s.logger
	}
	return logrus.StandardLogger()
}

// CloseSharedRedis releases the shared *redis.Client created by
// newConfiguredRedisComponents. Must be called exactly once during
// shutdown; it is a no-op when Redis was not configured.
func (s *chatServiceImpl) CloseSharedRedis() error {
	if s.closeSharedRedis == nil {
		return nil
	}
	return s.closeSharedRedis()
}

// WithEventLog overrides the event log on an existing chatServiceImpl.
// Test harnesses use this to inject a miniredis-backed log; production
// code defers to the env-gated constructor in NewChatService.
func (s *chatServiceImpl) WithEventLog(log RunEventLog) *chatServiceImpl {
	s.eventLog = log
	return s
}

func normalizeTitleJobQueue(queue TitleJobQueue) TitleJobQueue {
	if queue == nil {
		return nil
	}

	value := reflect.ValueOf(queue)
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		if value.IsNil() {
			return nil
		}
	case reflect.Invalid, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.String, reflect.Struct, reflect.UnsafePointer:
		// No-op. Non-nil kinds that cannot represent nil values.
	}

	return queue
}

func isNilTitleJobQueue(queue TitleJobQueue) bool {
	return normalizeTitleJobQueue(queue) == nil
}

func (s *chatServiceImpl) registerStreamCancel(sessionID uuid.UUID, cancel context.CancelFunc) {
	s.streamCancelMu.Lock()
	defer s.streamCancelMu.Unlock()
	if existing := s.activeStreamCancel[sessionID]; existing != nil {
		existing()
	}
	s.activeStreamCancel[sessionID] = cancel
}

func (s *chatServiceImpl) unregisterStreamCancel(sessionID uuid.UUID) {
	s.streamCancelMu.Lock()
	defer s.streamCancelMu.Unlock()
	delete(s.activeStreamCancel, sessionID)
}

// StopGeneration cancels the active stream for the session; no partial
// assistant message is persisted. The call is idempotent and tries three
// signals in order:
//
//  1. In-process: if this server has the streaming goroutine, we own the
//     cancel func directly and firing it immediately unblocks the executor.
//  2. Cross-process: the Redis run state may be owned by a different
//     worker / replica. Flip the cancel_requested flag so whichever worker
//     is driving the run sees it on its next tick and drives the Cancel
//     transition itself. Safe to call even when (1) already fired.
//
// When neither mechanism finds an active run the call succeeds silently
// (see ErrNoActiveRun handling in the controller) so clicking Stop on a
// run that has just finished is never an error from the user's view.
func (s *chatServiceImpl) StopGeneration(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.StopGeneration"
	s.streamCancelMu.Lock()
	cancel, ok := s.activeStreamCancel[sessionID]
	if ok {
		delete(s.activeStreamCancel, sessionID)
	}
	s.streamCancelMu.Unlock()
	if cancel != nil {
		cancel()
	}

	// Best-effort persist the cancel intent. If the active run lives on
	// another process (future: dedicated run worker, replica behind a
	// load balancer), this is the ONLY signal it will see.
	//
	// Use a detached context so the persisted cancel survives a client
	// disconnect that has already cancelled ctx.
	persistCtx := context.WithoutCancel(ctx)
	run, err := s.runState.GetPersistedRun(persistCtx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveRun) {
			// No active run — idempotent success from the user's view.
			return nil
		}
		// Any other persistence failure is surfaced so operators can
		// diagnose cross-process cancel delivery failures.
		return serrors.E(op, err)
	}
	if err := s.runState.RequestCancel(persistCtx, run.TenantID(), sessionID, run.ID()); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

// GetStreamStatus returns the active run for the session from memory or persisted state.
func (s *chatServiceImpl) GetStreamStatus(ctx context.Context, sessionID uuid.UUID) (*bichatservices.StreamStatus, error) {
	const op serrors.Op = "chatServiceImpl.GetStreamStatus"

	if run := s.runRegistry.GetBySession(sessionID); run != nil {
		run.Mu.RLock()
		content := run.Content
		runID := run.RunID
		startedAt := run.StartedAt
		run.Mu.RUnlock()
		meta := run.SnapshotMetadata()
		if startedAt.IsZero() {
			startedAt = time.Now()
		}
		return &bichatservices.StreamStatus{
			Active:    true,
			RunID:     runID,
			Snapshot:  bichatservices.StreamSnapshot{PartialContent: content, PartialMetadata: meta},
			StartedAt: startedAt,
		}, nil
	}

	// Fallback to persisted state (e.g. run may be in another process).
	run, err := s.getPersistedRun(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveRun) {
			return &bichatservices.StreamStatus{Active: false}, nil
		}
		return nil, serrors.E(op, err)
	}
	if run == nil {
		return &bichatservices.StreamStatus{Active: false}, nil
	}
	return &bichatservices.StreamStatus{
		Active:    true,
		RunID:     run.ID(),
		Snapshot:  bichatservices.StreamSnapshot{PartialContent: run.PartialContent(), PartialMetadata: run.PartialMetadata()},
		StartedAt: run.StartedAt(),
	}, nil
}

func (s *chatServiceImpl) createRunState(ctx context.Context, run domain.GenerationRun) (bool, error) {
	created, err := s.runState.CreateRunState(ctx, run)
	if err != nil || !created {
		return created, err
	}
	// Publish "streaming" to the per-tenant active run hash so sidebar
	// subscribers see the dot light up without waiting for the first
	// snapshot event. Best-effort: if Redis fan-out fails, the core
	// stream still works and the client can still pull /stream/status.
	if s.activeRunIndex != nil {
		_ = s.activeRunIndex.Upsert(ctx, run.TenantID(), ActiveRunStatus{
			SessionID: run.SessionID(),
			RunID:     run.ID(),
			Status:    string(domain.GenerationRunStatusStreaming),
			UpdatedAt: time.Now().UTC(),
		})
	}
	return created, nil
}

func (s *chatServiceImpl) createRunStateRecoveringOrphan(
	ctx context.Context,
	run domain.GenerationRun,
) (bool, error) {
	created, err := s.createRunState(ctx, run)
	if err == nil || !errors.Is(err, domain.ErrActiveRunExists) {
		return created, err
	}

	persistedRun, persistedErr := s.runState.GetPersistedRunForSession(
		ctx,
		run.TenantID(),
		run.SessionID(),
	)
	if persistedErr != nil {
		return false, err
	}
	databaseRun, databaseErr := s.chatRepo.GetActiveRunBySession(ctx, run.SessionID())
	if databaseErr != nil || databaseRun == nil {
		return false, err
	}

	// PostgreSQL owns the durable run lifecycle. CreateRun has already inserted
	// the new row in this transaction, so a different Redis run for the same
	// session can only be residue from a finalization that completed in SQL but
	// failed before clearing Redis. A genuinely concurrent run would have
	// prevented the PostgreSQL insert through its unique active-run constraint.
	if databaseRun.ID() != run.ID() || persistedRun.ID() == run.ID() {
		return false, err
	}

	if clearErr := s.finalizeRunState(
		ctx,
		run.TenantID(),
		run.SessionID(),
		persistedRun.ID(),
		string(domain.GenerationRunStatusFailed),
		s.runState.FailRunState,
	); clearErr != nil {
		return false, serrors.E("chatServiceImpl.createRunStateRecoveringOrphan", clearErr)
	}
	s.publishTerminalStatus(
		ctx,
		run.TenantID(),
		run.SessionID(),
		persistedRun.ID(),
		string(domain.GenerationRunStatusFailed),
	)
	s.log().WithFields(logrus.Fields{
		"tenant_id":          run.TenantID().String(),
		"session_id":         run.SessionID().String(),
		"orphaned_run_id":    persistedRun.ID().String(),
		"replacement_run_id": run.ID().String(),
	}).Warn("bichat: recovered orphaned redis generation run")

	return s.createRunState(ctx, run)
}

func (s *chatServiceImpl) getPersistedRun(ctx context.Context, sessionID uuid.UUID) (domain.GenerationRun, error) {
	return s.runState.GetPersistedRun(ctx, sessionID)
}

func (s *chatServiceImpl) getPersistedRunByID(ctx context.Context, runID uuid.UUID) (domain.GenerationRun, error) {
	return s.runState.GetPersistedRunByID(ctx, runID)
}

func (s *chatServiceImpl) updateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	return s.runState.UpdateRunSnapshot(ctx, tenantID, sessionID, runID, partialContent, partialMetadata)
}

func (s *chatServiceImpl) completeRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	if err := s.withinTx(context.WithoutCancel(ctx), func(txCtx context.Context) error {
		return s.chatRepo.CompleteRun(txCtx, runID)
	}); err != nil {
		return err
	}
	err := s.finalizeRunState(
		ctx,
		tenantID,
		sessionID,
		runID,
		string(domain.GenerationRunStatusCompleted),
		s.runState.CompleteRunState,
	)
	if err == nil {
		s.publishTerminalStatus(ctx, tenantID, sessionID, runID, string(domain.GenerationRunStatusCompleted))
	}
	return err
}

func (s *chatServiceImpl) cancelRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	if err := s.withinTx(context.WithoutCancel(ctx), func(txCtx context.Context) error {
		return s.chatRepo.CancelRun(txCtx, runID)
	}); err != nil {
		return err
	}
	err := s.finalizeRunState(
		ctx,
		tenantID,
		sessionID,
		runID,
		string(domain.GenerationRunStatusCancelled),
		s.runState.CancelRunState,
	)
	if err == nil {
		s.publishTerminalStatus(ctx, tenantID, sessionID, runID, string(domain.GenerationRunStatusCancelled))
	}
	return err
}

func (s *chatServiceImpl) finalizeRunState(
	ctx context.Context,
	tenantID, sessionID, runID uuid.UUID,
	terminalStatus string,
	finalize func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error,
) error {
	const op serrors.Op = "chatServiceImpl.finalizeRunState"

	var finalErr error
	for attempt := 1; attempt <= runStateFinalizationAttempts; attempt++ {
		finalErr = finalize(ctx, tenantID, sessionID, runID)
		if finalErr == nil {
			return nil
		}
		if attempt == runStateFinalizationAttempts || ctx.Err() != nil {
			break
		}

		timer := time.NewTimer(time.Duration(attempt) * runStateFinalizationRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			finalErr = errors.Join(finalErr, ctx.Err())
			attempt = runStateFinalizationAttempts
		case <-timer.C:
		}
	}
	if ctxErr := ctx.Err(); ctxErr != nil && !errors.Is(finalErr, ctxErr) {
		finalErr = errors.Join(finalErr, ctxErr)
	}

	s.log().WithError(finalErr).WithFields(logrus.Fields{
		"tenant_id":       tenantID.String(),
		"session_id":      sessionID.String(),
		"run_id":          runID.String(),
		"terminal_status": terminalStatus,
		"attempts":        runStateFinalizationAttempts,
	}).Error("bichat: failed to finalize generation run state")
	return serrors.E(op, finalErr)
}

// publishTerminalStatus is the single choke point for emitting the last
// sidebar status event. We publish then remove atomically so a snapshot
// fetched just after the terminal delta doesn't see a stale streaming
// entry (which would leave a dangling dot on the frontend).
func (s *chatServiceImpl) publishTerminalStatus(ctx context.Context, tenantID, sessionID, runID uuid.UUID, status string) {
	if s.activeRunIndex == nil {
		return
	}
	_ = s.activeRunIndex.PublishAndRemove(ctx, tenantID, ActiveRunStatus{
		SessionID: sessionID,
		RunID:     runID,
		Status:    status,
		UpdatedAt: time.Now().UTC(),
	})
}

type asyncRunWorker func(processCtx context.Context, persistCtx context.Context, runID uuid.UUID, session domain.Session, active *streamingsvc.ActiveRun)

func (s *chatServiceImpl) startAsyncRun(
	ctx context.Context,
	sessionID uuid.UUID,
	operation bichatservices.AsyncRunOperation,
	idempotencyKey string,
	prepare func(txCtx context.Context, session domain.Session) error,
	worker asyncRunWorker,
) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.startAsyncRun"

	var (
		session     domain.Session
		run         domain.GenerationRun
		err         error
		existingRun bool
	)
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err = s.chatRepo.GetSession(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}
		var runID uuid.UUID
		databaseRunReady := false
		if strings.TrimSpace(idempotencyKey) != "" {
			runID = continuationRunID(session.TenantID(), sessionID, idempotencyKey)
			run, err = s.chatRepo.GetRunByID(txCtx, runID)
			if err == nil {
				if run.SessionID() != sessionID || run.TenantID() != session.TenantID() {
					return serrors.E(op, serrors.KindValidation, "idempotent run belongs to another session")
				}
				switch run.Status() {
				case domain.GenerationRunStatusCompleted:
					existingRun = true
					return nil
				case domain.GenerationRunStatusStreaming:
					lastSeen := run.LastUpdatedAt()
					if lastSeen.IsZero() {
						lastSeen = run.StartedAt()
					}
					if !lastSeen.Before(time.Now().Add(-domain.GenerationRunStaleAfter)) {
						existingRun = true
						return nil
					}
				case domain.GenerationRunStatusCancelled, domain.GenerationRunStatusFailed:
					// Terminal failures are retryable under the same
					// idempotency key and deterministic run id.
				default:
					return serrors.E(op, serrors.KindValidation, "unsupported generation run status")
				}

				run, err = s.chatRepo.RestartRun(
					txCtx,
					runID,
					time.Now().Add(-domain.GenerationRunStaleAfter),
				)
				if err == nil {
					databaseRunReady = true
				} else {
					if !errors.Is(err, domain.ErrRunNotFound) {
						return serrors.E(op, err)
					}

					// Another process may have won the restart race. Re-read the
					// deterministic row and accept its live/completed result.
					run, err = s.chatRepo.GetRunByID(txCtx, runID)
					if err != nil {
						return serrors.E(op, err)
					}
					if run.SessionID() != sessionID || run.TenantID() != session.TenantID() {
						return serrors.E(op, serrors.KindValidation, "idempotent run belongs to another session")
					}
					if run.Status() == domain.GenerationRunStatusStreaming ||
						run.Status() == domain.GenerationRunStatusCompleted {
						existingRun = true
						return nil
					}
					return serrors.E(op, domain.ErrActiveRunExists)
				}
			}
			if err != nil && !errors.Is(err, domain.ErrRunNotFound) {
				return serrors.E(op, err)
			}
		}
		if !databaseRunReady {
			run, err = domain.NewGenerationRun(domain.GenerationRunSpec{
				ID:        runID,
				SessionID: sessionID,
				TenantID:  session.TenantID(),
				UserID:    session.UserID(),
			})
			if err != nil {
				return serrors.E(op, serrors.KindValidation, err)
			}
			if err := s.chatRepo.CreateRun(txCtx, run); err != nil {
				if strings.TrimSpace(idempotencyKey) != "" &&
					errors.Is(err, domain.ErrActiveRunExists) {
					concurrent, getErr := s.chatRepo.GetRunByID(txCtx, run.ID())
					if getErr == nil &&
						concurrent.SessionID() == sessionID &&
						concurrent.TenantID() == session.TenantID() &&
						(concurrent.Status() == domain.GenerationRunStatusStreaming ||
							concurrent.Status() == domain.GenerationRunStatusCompleted) {
						run = concurrent
						existingRun = true
						return nil
					}
				}
				return serrors.E(op, err)
			}
		}
		_, err = s.createRunStateRecoveringOrphan(txCtx, run)
		if err != nil {
			return serrors.E(op, err)
		}
		// Create the journal before accepting the run. HITL may produce no
		// chunks while the model is working; an absent key ends Redis tailing.
		if err := s.appendRunEvent(txCtx, session.TenantID(), sessionID, run.ID(), bichatservices.StreamChunk{
			Type: bichatservices.ChunkTypeStreamStarted, RunID: run.ID().String(), Timestamp: time.Now(),
		}); err != nil {
			return serrors.E(op, err)
		}
		if prepare != nil {
			if err := prepare(txCtx, session); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		if run != nil && session != nil {
			_ = s.cancelRunState(context.WithoutCancel(ctx), session.TenantID(), sessionID, run.ID())
		}
		return bichatservices.AsyncRunAccepted{}, serrors.E(op, err)
	}
	if existingRun {
		return bichatservices.AsyncRunAccepted{
			Accepted:  true,
			Operation: operation,
			SessionID: sessionID,
			RunID:     run.ID(),
			StartedAt: run.StartedAt(),
		}, nil
	}

	processCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))
	s.registerStreamCancel(sessionID, cancelProcess)

	active := streamingsvc.NewActiveRun(run.ID(), sessionID, cancelProcess, time.Now())
	s.runRegistry.Add(active)

	persistCtx := context.WithoutCancel(ctx)
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)
	s.mirrorRunEvents(persistCtx, session.TenantID(), active)
	go func() {
		defer s.expireRunEvents(persistCtx, session.TenantID(), sessionID, run.ID())
		worker(processCtx, persistCtx, run.ID(), session, active)
	}()

	return bichatservices.AsyncRunAccepted{
		Accepted:  true,
		Operation: operation,
		SessionID: sessionID,
		RunID:     run.ID(),
		StartedAt: active.StartedAt,
	}, nil
}

func continuationRunID(tenantID, sessionID uuid.UUID, idempotencyKey string) uuid.UUID {
	name := tenantID.String() + "/" + sessionID.String() + "/" + strings.TrimSpace(idempotencyKey)
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("iota-sdk/bichat/continuation/"+name))
}

// ResumeStream attaches to an active run and streams snapshot then new chunks.
func (s *chatServiceImpl) ResumeStream(ctx context.Context, sessionID uuid.UUID, runID uuid.UUID, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.ResumeStream"

	run := s.runRegistry.GetByRun(runID)
	if run != nil {
		if run.SessionID != sessionID {
			return serrors.E(op, serrors.KindValidation, "session id mismatch")
		}

		ch := make(chan bichatservices.StreamChunk, 256)
		run.Mu.RLock()
		partialContent := run.Content
		run.Mu.RUnlock()
		snap := bichatservices.StreamSnapshot{PartialContent: partialContent, PartialMetadata: run.SnapshotMetadata()}

		onChunk(bichatservices.StreamChunk{
			Type:      bichatservices.ChunkTypeSnapshot,
			Snapshot:  &snap,
			Timestamp: time.Now(),
		})

		run.AddSubscriber(ch)
		defer run.RemoveSubscriber(ch)

		for {
			select {
			case <-ctx.Done():
				return nil
			case chunk, ok := <-ch:
				if !ok {
					return nil
				}
				onChunk(chunk)
				if chunk.Type == bichatservices.ChunkTypeDone || chunk.Type == bichatservices.ChunkTypeError {
					return nil
				}
			}
		}
	}

	// Remote-node resume path: poll persisted run state by run id.
	persisted, err := s.getPersistedRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) || errors.Is(err, domain.ErrNoActiveRun) {
			return bichatservices.ErrRunNotFoundOrFinished
		}
		return serrors.E(op, err)
	}
	if persisted == nil {
		return bichatservices.ErrRunNotFoundOrFinished
	}
	if persisted.SessionID() != sessionID {
		return serrors.E(op, serrors.KindValidation, "session id mismatch")
	}

	lastContent := persisted.PartialContent()
	lastMetadata := persisted.PartialMetadata()
	onChunk(bichatservices.StreamChunk{
		Type: bichatservices.ChunkTypeSnapshot,
		Snapshot: &bichatservices.StreamSnapshot{
			PartialContent:  lastContent,
			PartialMetadata: lastMetadata,
		},
		Timestamp: time.Now(),
	})
	if persisted.Status() != domain.GenerationRunStatusStreaming {
		if persisted.Status() == domain.GenerationRunStatusCancelled {
			onChunk(streamingsvc.TerminalChunk(serrors.E(op, "generation cancelled"), 0))
		} else {
			onChunk(streamingsvc.TerminalChunk(nil, 0))
		}
		return nil
	}

	ticker := time.NewTicker(remoteResumePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			current, lookupErr := s.getPersistedRunByID(ctx, runID)
			if lookupErr != nil {
				if errors.Is(lookupErr, domain.ErrRunNotFound) || errors.Is(lookupErr, domain.ErrNoActiveRun) {
					onChunk(streamingsvc.TerminalChunk(nil, 0))
					return nil
				}
				return serrors.E(op, lookupErr)
			}
			if current == nil {
				onChunk(streamingsvc.TerminalChunk(nil, 0))
				return nil
			}
			if current.SessionID() != sessionID {
				return serrors.E(op, serrors.KindValidation, "session id mismatch")
			}

			currentContent := current.PartialContent()
			currentMetadata := current.PartialMetadata()
			contentChanged := currentContent != lastContent
			metadataChanged := !reflect.DeepEqual(currentMetadata, lastMetadata)
			if contentChanged || metadataChanged {
				if contentChanged && !metadataChanged && strings.HasPrefix(currentContent, lastContent) {
					delta := strings.TrimPrefix(currentContent, lastContent)
					if delta != "" {
						onChunk(bichatservices.StreamChunk{
							Type:      bichatservices.ChunkTypeContent,
							Content:   delta,
							Timestamp: time.Now(),
						})
					}
				} else {
					onChunk(bichatservices.StreamChunk{
						Type: bichatservices.ChunkTypeSnapshot,
						Snapshot: &bichatservices.StreamSnapshot{
							PartialContent:  currentContent,
							PartialMetadata: currentMetadata,
						},
						Timestamp: time.Now(),
					})
				}
				lastContent = currentContent
				lastMetadata = currentMetadata
			}

			if current.Status() != domain.GenerationRunStatusStreaming {
				if current.Status() == domain.GenerationRunStatusCancelled {
					onChunk(streamingsvc.TerminalChunk(serrors.E(op, "generation cancelled"), 0))
				} else {
					onChunk(streamingsvc.TerminalChunk(nil, 0))
				}
				return nil
			}
		}
	}
}

// TailRunEvents forwards events from the durable per-run Redis event log
// to the caller. It resolves tenantID from persisted run state (the HTTP
// controller only knows session + run ids and the Last-Event-ID header),
// replays all entries with stream id > from, and then live-tails the log
// until a terminal event, ctx cancellation, or TTL expiry.
//
// Returns:
//   - bichatservices.ErrRunEventLogUnavailable when Redis is not configured;
//   - bichatservices.ErrRunNotFoundOrFinished when the run is unknown;
//   - wrapped errors for the rest.
//
// onEvent is called synchronously from the goroutine that drives Tail and
// must not block; HTTP handlers typically write an SSE `id:` + `event:`
// + `data:` triple and flush.
func (s *chatServiceImpl) TailRunEvents(
	ctx context.Context,
	sessionID, runID uuid.UUID,
	from string,
	onEvent func(bichatservices.RunEventDelivery),
) error {
	const op serrors.Op = "chatServiceImpl.TailRunEvents"

	if s.eventLog == nil {
		return bichatservices.ErrRunEventLogUnavailable
	}

	// Load the persisted run within the tenant scope already carried by
	// ctx. After lookup succeeds, use the run's tenant id for event-log
	// replay/tailing. This endpoint must still be invoked with a context
	// that contains the current tenant.
	persisted, err := s.getPersistedRunByID(ctx, runID)
	if err != nil {
		if errors.Is(err, domain.ErrRunNotFound) || errors.Is(err, domain.ErrNoActiveRun) {
			return bichatservices.ErrRunNotFoundOrFinished
		}
		return serrors.E(op, err)
	}
	if persisted == nil {
		return bichatservices.ErrRunNotFoundOrFinished
	}
	if persisted.SessionID() != sessionID {
		return serrors.E(op, serrors.KindValidation, "session id mismatch")
	}
	tenantID := persisted.TenantID()

	// Step 1: replay missed events synchronously so the client receives
	// them in a deterministic order before live tailing begins.
	replayed, err := s.eventLog.Replay(ctx, tenantID, runID, from)
	if err != nil {
		return serrors.E(op, err)
	}
	lastID := from
	for _, evt := range replayed {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return serrors.E(op, bichatservices.ErrRunEventStreamInterrupted)
		}
		onEvent(bichatservices.RunEventDelivery{
			StreamID: evt.StreamID,
			Type:     evt.Type,
			Payload:  append([]byte(nil), evt.Payload...),
		})
		lastID = evt.StreamID
		if IsRunEventTerminal(evt.Type) {
			return nil
		}
	}

	// Step 2: live-tail from the last event id we delivered. Tail closes
	// the channel on terminal event / ctx cancel / TTL expiry.
	tailCh, err := s.eventLog.Tail(ctx, tenantID, runID, lastID)
	if err != nil {
		return serrors.E(op, err)
	}
	for evt := range tailCh {
		if err := ctx.Err(); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return serrors.E(op, bichatservices.ErrRunEventStreamInterrupted)
		}
		onEvent(bichatservices.RunEventDelivery{
			StreamID: evt.StreamID,
			Type:     evt.Type,
			Payload:  append([]byte(nil), evt.Payload...),
		})
		if IsRunEventTerminal(evt.Type) {
			return nil
		}
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return serrors.E(op, bichatservices.ErrRunEventStreamInterrupted)
}

// TailActiveRuns delivers the per-tenant sidebar view: snapshot rows
// first (each with Event="snapshot"), then live delta events from the
// active-run index pubsub (each with Event="update"). The handler
// blocks until ctx is cancelled or the pubsub connection breaks.
//
// Snapshot + Subscribe are ordered so a subscriber never misses a delta
// landed between HGETALL and pubsub establishment: we Subscribe first
// (the pubsub handshake is the barrier), then HGETALL the current
// state, then forward live deltas. If an update for session S lands
// between Subscribe and Snapshot, the snapshot row for S will be the
// more recent one.
func (s *chatServiceImpl) TailActiveRuns(ctx context.Context, onEvent func(bichatservices.ActiveRunDelivery)) error {
	const op serrors.Op = "chatServiceImpl.TailActiveRuns"

	if s.activeRunIndex == nil {
		return bichatservices.ErrActiveRunIndexUnavailable
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}

	subCh, err := s.activeRunIndex.Subscribe(ctx, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}

	// Snapshot AFTER Subscribe so we don't miss deltas published
	// between the two calls (see comment above).
	snap, err := s.activeRunIndex.Snapshot(ctx, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}

	// snapshotHighWaterMark tracks the highest UpdatedAt seen per session
	// in the snapshot phase. Any pubsub delta with UpdatedAt ≤ the
	// snapshot value for the same session is a buffered-but-stale delta
	// that arrived before we subscribed yet landed in the pubsub buffer
	// afterward. Forwarding it would regress the sidebar to an older
	// status, so we drop it.
	snapshotHighWaterMark := make(map[uuid.UUID]int64, len(snap))
	for _, entry := range snap {
		if err := ctx.Err(); err != nil {
			return nil //nolint:nilerr // context cancelled — clean stop, not an error
		}
		ms := entry.UpdatedAt.UnixMilli()
		if ms > snapshotHighWaterMark[entry.SessionID] {
			snapshotHighWaterMark[entry.SessionID] = ms
		}
		onEvent(bichatservices.ActiveRunDelivery{
			Event:     "snapshot",
			SessionID: entry.SessionID,
			RunID:     entry.RunID,
			Status:    entry.Status,
			UpdatedAt: ms,
		})
	}

	for entry := range subCh {
		if err := ctx.Err(); err != nil {
			return nil //nolint:nilerr // context cancelled — clean stop, not an error
		}
		ms := entry.UpdatedAt.UnixMilli()
		// Drop deltas that don't advance beyond the snapshot high-water
		// mark for this session — they are stale pubsub entries buffered
		// before the snapshot was taken.
		if ms <= snapshotHighWaterMark[entry.SessionID] {
			continue
		}
		// Advance the high-water mark so subsequent deltas for the same
		// session are also deduplicated correctly within the live window.
		snapshotHighWaterMark[entry.SessionID] = ms
		onEvent(bichatservices.ActiveRunDelivery{
			Event:     "update",
			SessionID: entry.SessionID,
			RunID:     entry.RunID,
			Status:    entry.Status,
			UpdatedAt: ms,
		})
	}
	return nil
}

// SendMessage sends a message to a session and processes it with the agent.
func (s *chatServiceImpl) SendMessage(ctx context.Context, req bichatservices.SendMessageRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.SendMessage"
	startedAt := time.Now()

	var session domain.Session
	var err error

	var authorUserID *int64
	if req.UserID != 0 {
		authorUserID = &req.UserID
	}
	userMsg, err := domain.NewUserMessage(domain.UserMessageSpec{
		SessionID:    req.SessionID,
		AuthorUserID: authorUserID,
		Content:      req.Content,
		Attachments:  req.Attachments,
	})
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	processCtx := bichatservices.WithArtifactMessageID(ctx, userMsg.ID())
	if req.ReasoningEffort != nil {
		processCtx = bichatservices.WithReasoningEffort(processCtx, *req.ReasoningEffort)
	}
	if req.Model != nil {
		processCtx = bichatservices.WithModelOverride(processCtx, *req.Model)
	}

	domainAttachments := cloneAttachmentsForMessage(userMsg.ID(), req.Attachments)

	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err = s.chatRepo.GetSession(txCtx, req.SessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		session, err = s.maybeReplaceHistoryFromMessage(txCtx, session, req.ReplaceFromMessageID)
		if err != nil {
			return serrors.E(op, err)
		}
		if err := s.ensureNoOpenQuestionForSend(txCtx, req.SessionID); err != nil {
			return err
		}

		if err := s.chatRepo.SaveMessage(txCtx, userMsg); err != nil {
			return serrors.E(op, err)
		}

		for _, att := range domainAttachments {
			msgID := userMsg.ID()
			artifact := domain.ArtifactSpec{
				TenantID:       session.TenantID(),
				SessionID:      session.ID(),
				MessageID:      &msgID,
				Type:           domain.ArtifactTypeAttachment,
				Name:           att.FileName(),
				MimeType:       att.MimeType(),
				URL:            att.FilePath(),
				SizeBytes:      att.SizeBytes(),
				Status:         domain.ArtifactStatusAvailable,
				IdempotencyKey: "attachment:" + msgID.String() + ":" + att.FileName(),
			}
			if att.UploadID() != nil {
				artifact.UploadID = att.UploadID()
			}
			artifactEntity, err := domain.NewArtifactFromSpec(artifact)
			if err != nil {
				return serrors.E(op, serrors.KindValidation, err)
			}
			if err := s.chatRepo.SaveArtifact(txCtx, artifactEntity); err != nil {
				return serrors.E(op, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Process message with agent
	gen, err := s.agentService.ProcessMessage(processCtx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(processCtx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var assistantMsg types.Message
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		assistantMsg, session, err = s.saveAgentResult(txCtx, op, session, req.SessionID, result, startedAt, req.Content)
		return err
	})
	if err != nil {
		return nil, err
	}

	if result.interrupt == nil {
		s.maybeGenerateTitleAsync(ctx, req.SessionID)
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      userMsg,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// SendMessageStream sends a message and streams the response via callback.
//
// Two execution modes:
//
//   - inline (default): the generation runs in a goroutine on this process
//     via RunExecutor.Execute; the requesting handler receives chunks over an
//     in-memory subscriber channel.
//   - enqueue (BICHAT_RUN_WORKERS_ENABLED=true, Redis required): the request
//     is handed to the Redis run queue and this handler becomes a pure
//     event-log cursor reader — the same Replay+Tail path as
//     GET /stream/events. A send that lands while the session already has an
//     active run is pushed onto the per-session FIFO and surfaces as a
//     "queued" sidebar status instead of failing with ErrActiveRunExists.
//
// Without Redis the flag has no effect: enqueue mode requires the queue and
// the event log, and the service falls back to inline execution otherwise.
func (s *chatServiceImpl) SendMessageStream(ctx context.Context, req bichatservices.SendMessageRequest, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.SendMessageStream"
	startedAt := time.Now()

	enqueueMode := s.runWorkersEnabled && s.runJobQueue != nil && s.eventLog != nil && s.runSessionQueue != nil

	// request_id dedupe: if the client supplied an idempotency key,
	// claim it before doing any real work. A duplicate send within
	// the ~30 min dedupe window converges on the existing run — we
	// emit stream_started with the existing run id then delegate to
	// the resume path so the second sender tails the same stream as
	// the first. This mirrors the cross-device UX from the plan.
	// When Redis is not configured (dev/CI) the queue is nil and
	// dedupe silently no-ops.
	var claimedRunID uuid.UUID
	// reqTenantID is used to scope the dedupe key. Resolved from context
	// (set by HTTP auth middleware); falls back to uuid.Nil when not set
	// (e.g. internal callers without a request context), in which case
	// dedupe is skipped safely.
	reqTenantID, _ := composables.UseTenantID(ctx)

	// releaseRequest drops the tenant-scoped dedupe mapping so that a
	// retry with the same requestID can mint a fresh run instead of
	// deduping to a phantom one. Called on early-exit error paths (before
	// the run is persisted) and at terminal run completion. Safe to call
	// multiple times: the Redis key may already be gone and the
	// implementation swallows redis.Nil.
	releaseRequest := func() {
		if req.RequestID != nil && s.runJobQueue != nil && reqTenantID != uuid.Nil {
			_ = s.runJobQueue.ReleaseRequest(context.WithoutCancel(ctx), reqTenantID, *req.RequestID)
		}
	}

	if req.RequestID != nil && s.runJobQueue != nil && reqTenantID != uuid.Nil {
		runID, deduped, claimErr := s.runJobQueue.ClaimRequest(ctx, reqTenantID, *req.RequestID, uuid.New())
		if claimErr == nil {
			if deduped {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeStreamStarted,
					RunID:     runID.String(),
					Timestamp: time.Now(),
				})
				if enqueueMode {
					// The run may be executing on another worker; the event
					// log is the only authoritative stream in enqueue mode.
					return s.tailRunEventsToChunks(ctx, req.SessionID, runID, RunEventStreamStart, onChunk)
				}
				return s.ResumeStream(ctx, req.SessionID, runID, onChunk)
			}
			claimedRunID = runID
		}
		// Claim error falls through without dedupe — a Redis blip must
		// not prevent the user's message from sending.
	}

	var session domain.Session
	var err error

	var authorUserID *int64
	if req.UserID != 0 {
		authorUserID = &req.UserID
	}
	userMsg, err := domain.NewUserMessage(domain.UserMessageSpec{
		SessionID:    req.SessionID,
		AuthorUserID: authorUserID,
		Content:      req.Content,
		Attachments:  req.Attachments,
	})
	if err != nil {
		return serrors.E(op, serrors.KindValidation, err)
	}

	domainAttachments := cloneAttachmentsForMessage(userMsg.ID(), req.Attachments)

	var run domain.GenerationRun
	runStateCreated := false

	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err = s.chatRepo.GetSession(txCtx, req.SessionID)
		if err != nil {
			return serrors.E(op, err)
		}
		session, err = s.maybeReplaceHistoryFromMessage(txCtx, session, req.ReplaceFromMessageID)
		if err != nil {
			return serrors.E(op, err)
		}
		if err := s.ensureNoOpenQuestionForSend(txCtx, req.SessionID); err != nil {
			return err
		}

		run, err = domain.NewGenerationRun(domain.GenerationRunSpec{
			// claimedRunID is the UUID reserved by ClaimRequest when a
			// request_id was supplied — preserving it means the dedupe
			// mapping points to the correct run (and a retry within
			// the dedupe window attaches correctly). uuid.Nil causes
			// NewGenerationRun to mint a fresh id.
			ID:        claimedRunID,
			SessionID: req.SessionID,
			TenantID:  session.TenantID(),
			UserID:    session.UserID(),
		})
		if err != nil {
			return serrors.E(op, serrors.KindValidation, err)
		}
		if s.runState.Enabled() {
			if err := s.chatRepo.CreateRun(txCtx, run); err != nil {
				return serrors.E(op, err)
			}
		}
		runStateCreated, err = s.createRunStateRecoveringOrphan(txCtx, run)
		if err != nil {
			return err
		}

		if err := s.chatRepo.SaveMessage(txCtx, userMsg); err != nil {
			return serrors.E(op, err)
		}

		for _, att := range domainAttachments {
			msgID := userMsg.ID()
			artifact := domain.ArtifactSpec{
				TenantID:       session.TenantID(),
				SessionID:      session.ID(),
				MessageID:      &msgID,
				Type:           domain.ArtifactTypeAttachment,
				Name:           att.FileName(),
				MimeType:       att.MimeType(),
				URL:            att.FilePath(),
				SizeBytes:      att.SizeBytes(),
				Status:         domain.ArtifactStatusAvailable,
				IdempotencyKey: "attachment:" + msgID.String() + ":" + att.FileName(),
			}
			if att.UploadID() != nil {
				artifact.UploadID = att.UploadID()
			}
			artifactEntity, err := domain.NewArtifactFromSpec(artifact)
			if err != nil {
				return serrors.E(op, serrors.KindValidation, err)
			}
			if err := s.chatRepo.SaveArtifact(txCtx, artifactEntity); err != nil {
				return serrors.E(op, err)
			}
		}

		return nil
	})
	if err != nil {
		if enqueueMode && session != nil && run != nil && errors.Is(err, domain.ErrActiveRunExists) {
			// Park behind the active run instead of failing. The whole tx
			// rolled back, so the message is not persisted yet: the queued
			// payload carries no user message id and the executor commits
			// it right before generating. The request claim is kept on
			// purpose so duplicate sends dedupe while the job waits; the
			// worker releases it at the run's terminal transition.
			return s.queueRunBehindActive(ctx, session, newRunJobPayload(req, session, run.ID(), uuid.Nil, domainAttachments), onChunk)
		}
		// Release the request_id dedupe mapping so the client can retry with the
		// same requestID and get a fresh run — the current run was never persisted.
		releaseRequest()
		if runStateCreated && run != nil && session != nil {
			_ = s.cancelRunState(context.WithoutCancel(ctx), session.TenantID(), req.SessionID, run.ID())
		}
		if errors.Is(err, domain.ErrActiveRunExists) {
			return serrors.E(op, err)
		}
		return err
	}
	// Decouple generation from request cancellation, but keep request values
	// (tenant/user/pool/tx) required by downstream services and repositories.
	persistCtx := context.WithoutCancel(ctx)
	// Stream finalization may outlive request-scoped middleware transactions.
	// Clear TxKey so persistence always opens its own durable transaction.
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)

	job := newRunJobPayload(req, session, run.ID(), userMsg.ID(), domainAttachments)
	job.EnqueuedAt = startedAt

	if enqueueMode {
		if _, enqueueErr := s.runJobQueue.EnqueueClaimed(persistCtx, job); enqueueErr == nil {
			// Journal the start marker so the cursor reader below (and any
			// later reconnect) sees a coherent event log from the beginning.
			_ = s.appendRunEvent(persistCtx, session.TenantID(), req.SessionID, run.ID(), bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeStreamStarted,
				RunID:     run.ID().String(),
				Timestamp: time.Now(),
			})
			onChunk(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeStreamStarted,
				RunID:     run.ID().String(),
				Timestamp: time.Now(),
			})
			// Pure cursor reader: replay + tail the durable log until the
			// worker-driven run reaches a terminal event. Persistence is
			// owned by the worker process, not this request.
			return s.tailRunEventsToChunks(ctx, req.SessionID, run.ID(), RunEventStreamStart, onChunk)
		} else {
			// Enqueue failed (Redis blip) — fall through to inline
			// execution so the user still gets an answer. The request
			// claim stays held and is released by the inline terminal
			// path below.
			s.log().WithError(serrors.E(op, enqueueErr)).
				WithField("session_id", req.SessionID.String()).
				WithField("run_id", run.ID().String()).
				Warn("bichat: run enqueue failed; falling back to inline execution")
		}
	}

	// Inline execution (also the enqueue failure fallback): the ActiveRun is
	// created here so the handler's subscriber channel is wired before the
	// executor's first broadcast; Execute reuses it via the run registry.
	processCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))
	s.registerStreamCancel(req.SessionID, cancelProcess)

	active := streamingsvc.NewActiveRun(run.ID(), req.SessionID, cancelProcess, time.Now())
	primaryCh := make(chan bichatservices.StreamChunk, 256)
	active.AddSubscriber(primaryCh)
	s.runRegistry.Add(active)

	s.mirrorRunEvents(persistCtx, session.TenantID(), active)

	go func() { _ = s.runExecutor.Execute(processCtx, job) }()

	onChunk(bichatservices.StreamChunk{
		Type:      bichatservices.ChunkTypeStreamStarted,
		RunID:     run.ID().String(),
		Timestamp: time.Now(),
	})

	var streamErr error
	for {
		select {
		case <-ctx.Done():
			active.RemoveSubscriber(primaryCh)
			return nil
		case chunk, ok := <-primaryCh:
			if !ok {
				return streamErr
			}
			onChunk(chunk)
			if chunk.Type == bichatservices.ChunkTypeDone {
				// Wait for run loop shutdown/persistence so request-scoped resources
				// (notably test transactions) are no longer in use before returning.
				for range primaryCh {
				}
				// Release request_id dedupe mapping at terminal success so a retry
				// with the same requestID starts a new run rather than deduping to
				// the completed one.
				releaseRequest()
				return nil
			}
			if chunk.Type == bichatservices.ChunkTypeError {
				streamErr = chunk.Error
				// Drain until channel closes so the goroutine can persist and exit
				for range primaryCh {
				}
				// Release on terminal error so the client can retry with the same
				// requestID and get a fresh attempt.
				releaseRequest()
				return streamErr
			}
		}
	}
}

// newRunJobPayload serialises a send request into the self-contained job
// handed to the run queue (enqueue mode) or to the in-process executor
// (inline mode). Attachments travel as upload ids; the executor re-resolves
// them from PostgreSQL so binary references never enter Redis.
func newRunJobPayload(req bichatservices.SendMessageRequest, session domain.Session, runID, userMessageID uuid.UUID, attachments []domain.Attachment) RunJobPayload {
	uploadIDs := make([]int64, 0, len(attachments))
	for _, att := range attachments {
		if id := att.UploadID(); id != nil {
			uploadIDs = append(uploadIDs, *id)
		}
	}
	requestID := uuid.Nil
	if req.RequestID != nil {
		requestID = *req.RequestID
	}
	return RunJobPayload{
		TenantID:             session.TenantID(),
		SessionID:            req.SessionID,
		UserID:               req.UserID,
		RequestID:            requestID,
		RunID:                runID,
		UserMessageID:        userMessageID,
		Content:              req.Content,
		UploadIDs:            uploadIDs,
		ReplaceFromMessageID: req.ReplaceFromMessageID,
		ReasoningEffort:      req.ReasoningEffort,
		Model:                req.Model,
		DebugMode:            req.DebugMode,
	}
}

// queueRunBehindActive parks a send that landed while the session already
// has an active run. The job waits on the per-session FIFO; when the active
// run terminates, the executor (or the reaper) promotes it onto the run job
// stream. The caller's SSE response closes after stream_started — the
// sidebar carries the queued status until the queued run starts producing
// events under the same run id.
func (s *chatServiceImpl) queueRunBehindActive(ctx context.Context, session domain.Session, job RunJobPayload, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.queueRunBehindActive"

	if err := s.runSessionQueue.Push(ctx, session.TenantID(), job.SessionID, QueuedRunJob{Payload: job}); err != nil {
		// Degrade to the pre-queue behaviour: surface the conflict so the
		// client can retry, and release the claim so that retry mints a
		// fresh run instead of deduping onto a phantom one.
		if job.RequestID != uuid.Nil {
			_ = s.runJobQueue.ReleaseRequest(context.WithoutCancel(ctx), session.TenantID(), job.RequestID)
		}
		return serrors.E(op, domain.ErrActiveRunExists)
	}
	if s.activeRunIndex != nil {
		// Queued entries are intentionally not written to the generation
		// run store: the reaper only reaps streaming runs, so a job waiting
		// behind a long generation is never reaped.
		_ = s.activeRunIndex.Upsert(ctx, session.TenantID(), ActiveRunStatus{
			SessionID: job.SessionID,
			RunID:     job.RunID,
			Status:    ActiveRunStatusQueued,
			UpdatedAt: time.Now().UTC(),
		})
	}
	_ = s.appendRunEvent(ctx, session.TenantID(), job.SessionID, job.RunID, bichatservices.StreamChunk{
		Type:      bichatservices.ChunkTypeStreamStarted,
		RunID:     job.RunID.String(),
		Timestamp: time.Now(),
	})
	onChunk(bichatservices.StreamChunk{
		Type:      bichatservices.ChunkTypeStreamStarted,
		RunID:     job.RunID.String(),
		Timestamp: time.Now(),
	})
	return nil
}

// tailRunEventsToChunks forwards the durable run event log to onChunk as
// decoded stream chunks. It is the enqueue-mode response path: the same
// Replay+Tail machinery GET /stream/events uses, wrapped in the
// SendMessageStream chunk callback contract.
func (s *chatServiceImpl) tailRunEventsToChunks(ctx context.Context, sessionID, runID uuid.UUID, from string, onChunk func(bichatservices.StreamChunk)) error {
	return s.TailRunEvents(ctx, sessionID, runID, from, func(evt bichatservices.RunEventDelivery) {
		chunk, err := decodeRunEventChunk(evt.Payload)
		if err != nil {
			return
		}
		onChunk(chunk)
	})
}
