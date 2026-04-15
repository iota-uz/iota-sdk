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
	hitlsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/hitl"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const streamPersistenceTimeout = 10 * time.Second
const titleGenerationFallbackTimeout = 15 * time.Second
const streamSnapshotThrottle = 2 * time.Second
const remoteResumePollInterval = time.Second

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
	// runJobQueue is used only for its ClaimRequest side (not XAdd):
	// request_id idempotency in the inline SendMessageStream path.
	// nil when Redis is unconfigured → dedupe silently degrades to
	// the pre-existing "two concurrent sends on same session → second
	// fails ErrActiveRunExists" behaviour.
	runJobQueue        *RedisRunJobQueue
	streamCancelMu     sync.Mutex
	activeStreamCancel map[uuid.UUID]context.CancelFunc
	runRegistry        *streamingsvc.RunRegistry
}

// NewChatService creates a production implementation of ChatService.
//
// Example:
//
//	service := NewChatService(chatRepo, agentService, model, titleService, titleQueue)
func NewChatService(
	chatRepo domain.ChatRepository,
	agentService bichatservices.AgentService,
	model agents.Model,
	titleService TitleService,
	titleQueue TitleJobQueue,
) *chatServiceImpl {
	runStore := newConfiguredGenerationRunStore()
	accessRepo := chatRepo.(domain.SessionAccessRepository)
	// Use a single shared Redis connection for all Redis-backed components so
	// the process dials only one connection to Redis. Falls back to nil/noop
	// when REDIS_URL is unset (dev/CI without Redis).
	eventLog, activeRunIndex, runJobQueue := newConfiguredRedisComponents()
	return &chatServiceImpl{
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
		activeStreamCancel: make(map[uuid.UUID]context.CancelFunc),
		runRegistry:        streamingsvc.NewRunRegistry(),
	}
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
	// load balancer), this is the ONLY signal it will see. Failures
	// here are logged via serrors and swallowed so the in-process
	// cancel that already succeeded stays authoritative.
	run, err := s.runState.GetPersistedRun(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoActiveRun) {
			return nil
		}
		return nil
	}
	_ = s.runState.RequestCancel(ctx, run.TenantID(), sessionID, run.ID())
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
	err := s.runState.CompleteRunState(ctx, tenantID, sessionID, runID)
	s.publishTerminalStatus(ctx, tenantID, sessionID, runID, string(domain.GenerationRunStatusCompleted))
	return err
}

func (s *chatServiceImpl) cancelRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	err := s.runState.CancelRunState(ctx, tenantID, sessionID, runID)
	s.publishTerminalStatus(ctx, tenantID, sessionID, runID, string(domain.GenerationRunStatusCancelled))
	return err
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
	prepare func(txCtx context.Context, session domain.Session) error,
	worker asyncRunWorker,
) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.startAsyncRun"

	var (
		session         domain.Session
		run             domain.GenerationRun
		err             error
		runStateCreated bool
	)
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err = s.chatRepo.GetSession(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}
		run, err = domain.NewGenerationRun(domain.GenerationRunSpec{
			SessionID: sessionID,
			TenantID:  session.TenantID(),
			UserID:    session.UserID(),
		})
		if err != nil {
			return serrors.E(op, serrors.KindValidation, err)
		}
		runStateCreated, err = s.createRunState(txCtx, run)
		if err != nil {
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
		if runStateCreated && run != nil && session != nil {
			_ = s.cancelRunState(context.WithoutCancel(ctx), session.TenantID(), sessionID, run.ID())
		}
		return bichatservices.AsyncRunAccepted{}, serrors.E(op, err)
	}

	processCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))
	s.registerStreamCancel(sessionID, cancelProcess)

	active := streamingsvc.NewActiveRun(run.ID(), sessionID, cancelProcess, time.Now())
	s.runRegistry.Add(active)

	persistCtx := context.WithoutCancel(ctx)
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)
	go worker(processCtx, persistCtx, run.ID(), session, active)

	return bichatservices.AsyncRunAccepted{
		Accepted:  true,
		Operation: operation,
		SessionID: sessionID,
		RunID:     run.ID(),
		StartedAt: active.StartedAt,
	}, nil
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

	// Resolve tenant via persisted run state. The run entry carries the
	// tenant id so we don't have to plumb it through the request — the
	// GET /stream/events endpoint is browser-hit and can't be trusted
	// to carry a tenant header.
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
			return nil
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
			return nil
		}
		onEvent(bichatservices.RunEventDelivery{
			StreamID: evt.StreamID,
			Type:     evt.Type,
			Payload:  append([]byte(nil), evt.Payload...),
		})
	}
	return nil
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
		return bichatservices.ErrRunEventLogUnavailable
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
	for _, entry := range snap {
		if err := ctx.Err(); err != nil {
			return nil
		}
		onEvent(bichatservices.ActiveRunDelivery{
			Event:     "snapshot",
			SessionID: entry.SessionID,
			RunID:     entry.RunID,
			Status:    entry.Status,
			UpdatedAt: entry.UpdatedAt.UnixMilli(),
		})
	}

	for entry := range subCh {
		if err := ctx.Err(); err != nil {
			return nil
		}
		onEvent(bichatservices.ActiveRunDelivery{
			Event:     "update",
			SessionID: entry.SessionID,
			RunID:     entry.RunID,
			Status:    entry.Status,
			UpdatedAt: entry.UpdatedAt.UnixMilli(),
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
func (s *chatServiceImpl) SendMessageStream(ctx context.Context, req bichatservices.SendMessageRequest, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.SendMessageStream"
	startedAt := time.Now()

	// request_id dedupe: if the client supplied an idempotency key,
	// claim it before doing any real work. A duplicate send within
	// the ~30 min dedupe window converges on the existing run — we
	// emit stream_started with the existing run id then delegate to
	// the resume path so the second sender tails the same stream as
	// the first. This mirrors the cross-device UX from the plan.
	// When Redis is not configured (dev/CI) the queue is nil and
	// dedupe silently no-ops.
	var claimedRunID uuid.UUID
	if req.RequestID != nil && s.runJobQueue != nil {
		runID, deduped, claimErr := s.runJobQueue.ClaimRequest(ctx, *req.RequestID, uuid.New())
		if claimErr == nil {
			if deduped {
				onChunk(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeStreamStarted,
					RunID:     runID.String(),
					Timestamp: time.Now(),
				})
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
		runStateCreated, err = s.createRunState(txCtx, run)
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
	processCtx, cancelProcess := context.WithCancel(context.WithoutCancel(ctx))
	s.registerStreamCancel(req.SessionID, cancelProcess)

	processCtx = bichatservices.WithArtifactMessageID(processCtx, userMsg.ID())
	if req.ReasoningEffort != nil {
		processCtx = bichatservices.WithReasoningEffort(processCtx, *req.ReasoningEffort)
	}
	if req.Model != nil {
		processCtx = bichatservices.WithModelOverride(processCtx, *req.Model)
	}

	active := streamingsvc.NewActiveRun(run.ID(), req.SessionID, cancelProcess, time.Now())
	primaryCh := make(chan bichatservices.StreamChunk, 256)
	active.AddSubscriber(primaryCh)
	s.runRegistry.Add(active)

	persistCtx := context.WithoutCancel(ctx)
	// Stream finalization may outlive request-scoped middleware transactions.
	// Clear TxKey so persistence always opens its own durable transaction.
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)

	// When a Redis event log is configured, mirror every broadcast into
	// bichat:run-events:{tenant}:{run_id} so out-of-process readers (the
	// SSE controller on a reconnect, or another replica of the server)
	// can tail/replay via Last-Event-ID. Mirror failures are logged
	// implicitly by the log implementation and deliberately do not break
	// the in-memory path — an error appending to Redis should not abort
	// a live agent streaming to its primary client.
	if s.eventLog != nil {
		tenantID := session.TenantID()
		runID := run.ID()
		active.SetMirror(func(chunk bichatservices.StreamChunk) {
			eventType, body, err := encodeRunEventFromChunk(chunk)
			if err != nil {
				return
			}
			_, _ = s.eventLog.Append(persistCtx, tenantID, runID, RunEvent{
				Type:    eventType,
				Payload: body,
			})
		})
	}

	go s.runStreamLoop(processCtx, persistCtx, run.ID(), req, session, domainAttachments, startedAt, active)

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
				return nil
			}
			if chunk.Type == bichatservices.ChunkTypeError {
				streamErr = chunk.Error
				// Drain until channel closes so the goroutine can persist and exit
				for range primaryCh {
				}
				return streamErr
			}
		}
	}
}

// runStreamLoop runs the agent in a goroutine, updates active run state, and persists on completion or cancel.
func (s *chatServiceImpl) runStreamLoop(
	processCtx context.Context,
	persistCtx context.Context,
	runID uuid.UUID,
	req bichatservices.SendMessageRequest,
	session domain.Session,
	domainAttachments []domain.Attachment,
	startedAt time.Time,
	active *streamingsvc.ActiveRun,
) {
	const op serrors.Op = "chatServiceImpl.runStreamLoop"
	// Cleanup order matters: cancel first so generator work stops before closing
	// subscriber channels and unregistering/removing run bookkeeping.
	defer func() {
		if active.Cancel != nil {
			active.Cancel()
		}
		active.CloseAllSubscribers()
		s.runRegistry.Remove(active.RunID)
		s.unregisterStreamCancel(req.SessionID)
		// Shorten the run-events stream TTL now that the run is done; the
		// reaper doesn't need it any more and long-lived Redis keys for
		// every historical run would bloat memory unnecessarily. 5 min
		// gives slow reconnecting clients a small grace window.
		if s.eventLog != nil && session != nil {
			_ = s.eventLog.DropAfterTerminal(persistCtx, session.TenantID(), runID, 5*time.Minute)
		}
	}()

	gen, err := s.agentService.ProcessMessage(processCtx, req.SessionID, req.Content, domainAttachments)
	if err != nil {
		active.Broadcast(streamingsvc.TerminalChunk(err, 0))
		_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
		return
	}
	defer gen.Close()

	var interrupt *bichatservices.Interrupt
	var interruptAgentName string
	var providerResponseID *string
	var finalUsage *types.DebugUsage
	var generationMs int64
	var traceID string
	var requestID string
	var model string
	var provider string
	var finishReason string
	var thinking strings.Builder
	var observationReason string
	emitDoneChunk := false

	for {
		event, err := gen.Next(processCtx)
		if errors.Is(err, types.ErrGeneratorDone) {
			break
		}
		if err != nil {
			active.Broadcast(streamingsvc.TerminalChunk(err, 0))
			break
		}

		chunk := bichatservices.StreamChunk{Timestamp: time.Now()}

		switch event.Type {
		case agents.EventTypeContent:
			active.Mu.Lock()
			active.Content += event.Content
			active.Mu.Unlock()
			chunk.Type = bichatservices.ChunkTypeContent
			chunk.Content = event.Content
			active.Broadcast(chunk)

		case agents.EventTypeTextBlockEnd:
			active.Mu.Lock()
			// Record the byte offset of the segment boundary so resume
			// snapshots can split the accumulated content back into the
			// blocks the user originally saw.
			active.TextBlockOffsets = append(active.TextBlockOffsets, len(active.Content))
			active.Mu.Unlock()
			chunk.Type = bichatservices.ChunkTypeTextBlockEnd
			chunk.TextBlockSeq = event.TextBlockSeq
			active.Broadcast(chunk)

		case agents.EventTypeToolStart:
			active.Mu.Lock()
			recordToolEvent(active.ToolCalls, &active.ToolOrder, event.Tool)
			if event.Tool != nil && len(event.Tool.Artifacts) > 0 {
				recordToolArtifacts(active.ArtifactMap, event.Tool.Artifacts)
			}
			active.Mu.Unlock()
			if event.Tool != nil {
				chunk.Type = bichatservices.ChunkTypeToolStart
				chunk.Tool = agentToolToServiceTool(event.Tool)
				active.Broadcast(chunk)
			}

		case agents.EventTypeToolEnd:
			active.Mu.Lock()
			recordToolEvent(active.ToolCalls, &active.ToolOrder, event.Tool)
			if event.Tool != nil && len(event.Tool.Artifacts) > 0 {
				recordToolArtifacts(active.ArtifactMap, event.Tool.Artifacts)
			}
			active.Mu.Unlock()
			if event.Tool != nil {
				chunk.Type = bichatservices.ChunkTypeToolEnd
				chunk.Tool = agentToolToServiceTool(event.Tool)
				active.Broadcast(chunk)
			}

		case agents.EventTypeInterrupt:
			if event.ParsedInterrupt == nil {
				continue
			}
			pi := event.ParsedInterrupt
			questions := hitlsvc.AgentQuestionsToServiceQuestions(pi.Questions)
			interrupt = &bichatservices.Interrupt{CheckpointID: pi.CheckpointID, Questions: questions}
			interruptAgentName = pi.AgentName
			if interruptAgentName == "" {
				interruptAgentName = "default-agent"
			}
			providerResponseID = optionalStringPtr(pi.ProviderResponseID)
			chunk.Type = bichatservices.ChunkTypeInterrupt
			chunk.Interrupt = &bichatservices.InterruptEvent{
				CheckpointID:       pi.CheckpointID,
				AgentName:          pi.AgentName,
				ProviderResponseID: pi.ProviderResponseID,
				Questions:          questions,
			}
			active.Broadcast(chunk)

		case agents.EventTypeDone:
			providerResponseID = optionalStringPtr(event.ProviderResponseID)
			if event.Result != nil {
				if event.Result.TraceID != "" {
					traceID = event.Result.TraceID
				}
				requestID = event.Result.RequestID
				model = event.Result.Model
				provider = event.Result.Provider
				finishReason = event.Result.FinishReason
				if event.Result.Thinking != "" {
					thinking.Reset()
					thinking.WriteString(event.Result.Thinking)
				}
			}
			active.Mu.Lock()
			recordToolArtifacts(active.ArtifactMap, collectCodeInterpreterArtifacts(event.CodeInterpreter, event.FileAnnotations))
			active.Mu.Unlock()
			if event.Usage != nil {
				finalUsage = event.Usage
				active.Broadcast(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeUsage,
					Usage:     event.Usage,
					Timestamp: time.Now(),
				})
			}
			generationMs = time.Since(startedAt).Milliseconds()
			emitDoneChunk = true

		case agents.EventTypeThinking:
			if event.Content != "" {
				thinking.WriteString(event.Content)
			}
			chunk.Type = bichatservices.ChunkTypeThinking
			chunk.Content = event.Content
			active.Broadcast(chunk)

		case agents.EventTypeError:
			chunk.Type = bichatservices.ChunkTypeError
			chunk.Error = event.Error
			active.Broadcast(chunk)
		}

		active.Mu.RLock()
		shouldPersistSnapshot := time.Since(active.LastPersist) >= streamSnapshotThrottle
		content := active.Content
		active.Mu.RUnlock()
		if shouldPersistSnapshot {
			meta := active.SnapshotMetadata()
			_ = s.updateRunSnapshot(persistCtx, session.TenantID(), req.SessionID, runID, content, meta)
			// Refresh heartbeat at the snapshot throttle cadence (2s).
			// The reaper marks runs whose heartbeat is older than ~60s as
			// failed, so a 2s cadence leaves comfortable headroom under
			// slow LLM calls or tool executions.
			_ = s.runState.Heartbeat(persistCtx, session.TenantID(), req.SessionID, runID)
			// Check the out-of-band cancel flag. A Stop RPC from another
			// tab / device sets this flag on the persisted run; we
			// observe it here and wind the generator down by cancelling
			// processCtx. The next gen.Next will return ctx.Err() and
			// the outer loop's cleanup will emit a terminal chunk.
			if persistedRun, err := s.runState.GetPersistedRun(persistCtx, req.SessionID); err == nil && persistedRun != nil {
				if persistedRun.CancelRequested() && active.Cancel != nil {
					active.Cancel()
				}
			}
			active.Mu.Lock()
			active.LastPersist = time.Now()
			active.Mu.Unlock()
		}
	}

	if processCtx.Err() != nil {
		active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processCtx.Err()), 0))
		_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
		return
	}

	active.Mu.RLock()
	assistantContent := active.Content
	savedToolCalls := orderedToolCalls(active.ToolCalls, active.ToolOrder)
	artifactMap := mapsValues(active.ArtifactMap)
	active.Mu.RUnlock()

	if observationReason == "" && assistantContent == "" && len(savedToolCalls) == 0 {
		observationReason = "empty_assistant_output"
	}
	var assistantDebugTrace *types.DebugTrace
	if debugTrace := buildDebugTrace(
		req.SessionID,
		traceID,
		savedToolCalls,
		finalUsage,
		generationMs,
		thinking.String(),
		observationReason,
		model,
		provider,
		requestID,
		finishReason,
		req.Content,
		assistantContent,
		startedAt,
	); debugTrace != nil {
		assistantDebugTrace = debugTrace
	}
	var assistantQuestionData *types.QuestionData
	if interrupt != nil {
		qd, err := hitlsvc.BuildQuestionData(interrupt.CheckpointID, interruptAgentName, interrupt.Questions)
		if err == nil && qd != nil {
			assistantQuestionData = qd
		}
	}

	assistantMsg, err := domain.NewAssistantMessage(domain.AssistantMessageSpec{
		SessionID:    req.SessionID,
		Content:      assistantContent,
		ToolCalls:    savedToolCalls,
		DebugTrace:   assistantDebugTrace,
		QuestionData: assistantQuestionData,
	})
	if err != nil {
		active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, serrors.KindValidation, err), 0))
		_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
		return
	}
	persistCtx, persistCancel := context.WithTimeout(persistCtx, streamPersistenceTimeout)
	defer persistCancel()

	err = s.withinTx(persistCtx, func(txCtx context.Context) error {
		if err := s.chatRepo.SaveMessage(txCtx, assistantMsg); err != nil {
			return serrors.E(op, err)
		}
		if err := s.persistGeneratedArtifacts(txCtx, session, assistantMsg.ID(), artifactMap); err != nil {
			return serrors.E(op, err)
		}
		session = session.SetPreviousResponseID(providerResponseID, time.Now())
		if err := s.chatRepo.UpdateSession(txCtx, session); err != nil {
			return serrors.E(op, err)
		}
		return nil
	})
	if err != nil {
		active.Broadcast(streamingsvc.TerminalChunk(err, 0))
		runStateCtx, runStateCancel := context.WithTimeout(context.Background(), streamPersistenceTimeout)
		defer runStateCancel()
		_ = s.cancelRunState(runStateCtx, session.TenantID(), req.SessionID, runID)
		return
	}

	runStateCtx, runStateCancel := context.WithTimeout(context.Background(), streamPersistenceTimeout)
	defer runStateCancel()
	_ = s.completeRunState(runStateCtx, session.TenantID(), req.SessionID, runID)
	if emitDoneChunk {
		active.Broadcast(streamingsvc.TerminalChunk(nil, generationMs))
	}
	if interrupt == nil {
		s.maybeGenerateTitleAsync(persistCtx, req.SessionID)
	}
}
