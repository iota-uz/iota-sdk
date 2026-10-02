// Package services provides this package.
package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/google/uuid"
	hitlsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/hitl"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

// ErrRunBusy signals that the session gained another active run between a
// queued job's promotion and its execution (the promoted job lost the
// CreateRun race). The worker parks the job back at the head of the FIFO
// instead of retrying or failing it — the new run's terminal transition
// re-promotes it.
var ErrRunBusy = errors.New("bichat: session has an active run")

// RunExecutor executes one generation turn from a self-contained
// RunJobPayload. Both execution modes funnel through it: the inline POST
// path spawns a goroutine calling Execute in the same process, and the
// Redis run worker calls Execute for jobs consumed off bichat:run:jobs.
type RunExecutor interface {
	Execute(ctx context.Context, job RunJobPayload) error
}

// chatRunExecutor implements RunExecutor on top of the chat service's
// dependencies (chat repo, agent service, run state, event log).
type chatRunExecutor struct {
	svc *chatServiceImpl
}

func newChatRunExecutor(svc *chatServiceImpl) *chatRunExecutor {
	return &chatRunExecutor{svc: svc}
}

// Execute runs a full generation turn: it assembles the turn context from
// the payload (resolving the session and, for queued jobs whose message was
// never committed, the user message + artifacts + run row), then streams
// agent events into the ActiveRun, persists the assistant message, and
// drives the run to a terminal state.
//
// When the run was prepared by an inline HTTP send the ActiveRun already
// exists in the registry with the requesting handler's subscribers wired;
// it is reused so those subscribers see the first broadcast. A worker (or
// any other standalone caller) gets a fresh ActiveRun created here.
func (e *chatRunExecutor) Execute(ctx context.Context, job RunJobPayload) error {
	const op serrors.Op = "chatRunExecutor.Execute"

	if job.TenantID == uuid.Nil || job.SessionID == uuid.Nil || job.RunID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant, session and run ids are required")
	}

	svc := e.svc
	attachments, err := resolveUploadAttachments(ctx, job.UploadIDs)
	if err != nil {
		return serrors.E(op, err)
	}

	var (
		session       domain.Session
		artifactMsgID uuid.UUID
	)
	if job.UserMessageID == uuid.Nil {
		// Late-bind path: the job was queued behind an active run before
		// its user message and run row were committed, so this executor
		// owns the whole prepare transaction.
		var userMsg types.Message
		prepareErr := svc.withinTx(ctx, func(txCtx context.Context) error {
			s, err := svc.chatRepo.GetSession(txCtx, job.SessionID)
			if err != nil {
				return serrors.E(op, err)
			}
			session = s
			session, err = svc.maybeReplaceHistoryFromMessage(txCtx, session, job.ReplaceFromMessageID)
			if err != nil {
				return serrors.E(op, err)
			}
			if err := svc.ensureNoOpenQuestionForSend(txCtx, job.SessionID); err != nil {
				return err
			}

			var authorUserID *int64
			if job.UserID != 0 {
				authorUserID = &job.UserID
			}
			userMsg, err = domain.NewUserMessage(domain.UserMessageSpec{
				SessionID:    job.SessionID,
				AuthorUserID: authorUserID,
				Content:      job.Content,
				Attachments:  attachments,
			})
			if err != nil {
				return serrors.E(op, serrors.KindValidation, err)
			}
			if err := svc.chatRepo.SaveMessage(txCtx, userMsg); err != nil {
				return serrors.E(op, err)
			}
			if err := svc.saveAttachmentArtifacts(txCtx, session, userMsg.ID(), attachments); err != nil {
				return err
			}

			run, getErr := svc.chatRepo.GetRunByID(txCtx, job.RunID)
			if errors.Is(getErr, domain.ErrRunNotFound) {
				run, err = domain.NewGenerationRun(domain.GenerationRunSpec{
					ID:        job.RunID,
					SessionID: job.SessionID,
					TenantID:  session.TenantID(),
					UserID:    session.UserID(),
				})
				if err != nil {
					return serrors.E(op, serrors.KindValidation, err)
				}
				if err := svc.chatRepo.CreateRun(txCtx, run); err != nil {
					if errors.Is(err, domain.ErrActiveRunExists) {
						// A newer send won the session while this job sat in
						// the stream — re-queue it instead of failing it.
						return fmt.Errorf("%w: %w", ErrRunBusy, err)
					}
					return serrors.E(op, err)
				}
			} else if getErr != nil {
				return serrors.E(op, getErr)
			}
			if _, err := svc.createRunStateRecoveringOrphan(txCtx, run); err != nil {
				if errors.Is(err, domain.ErrActiveRunExists) {
					return fmt.Errorf("%w: %w", ErrRunBusy, err)
				}
				return serrors.E(op, err)
			}
			return nil
		})
		if prepareErr != nil {
			return prepareErr
		}
		artifactMsgID = userMsg.ID()
	} else {
		session, err = svc.chatRepo.GetSession(ctx, job.SessionID)
		if err != nil {
			return serrors.E(op, err)
		}
		artifactMsgID = job.UserMessageID
	}

	startedAt := job.EnqueuedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}

	processCtx := ctx
	active := svc.runRegistry.GetByRun(job.RunID)
	ownsActive := active == nil
	if ownsActive {
		var cancelProcess context.CancelFunc
		processCtx, cancelProcess = context.WithCancel(context.WithoutCancel(ctx))
		svc.registerStreamCancel(job.SessionID, cancelProcess)
		active = streamingsvc.NewActiveRun(job.RunID, job.SessionID, cancelProcess, startedAt)
		svc.runRegistry.Add(active)
	}
	processCtx = bichatservices.WithArtifactMessageID(processCtx, artifactMsgID)
	if job.ReasoningEffort != nil {
		processCtx = bichatservices.WithReasoningEffort(processCtx, *job.ReasoningEffort)
	}
	if job.Model != nil {
		processCtx = bichatservices.WithModelOverride(processCtx, *job.Model)
	}
	if job.DebugMode {
		processCtx = bichatservices.WithDebugMode(processCtx, true)
	}

	persistCtx := context.WithoutCancel(ctx)
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)

	if ownsActive {
		svc.mirrorRunEvents(persistCtx, session.TenantID(), active)
	}

	e.executeTurn(turnExecution{
		processCtx:  processCtx,
		persistCtx:  persistCtx,
		job:         job,
		session:     session,
		attachments: attachments,
		startedAt:   startedAt,
		active:      active,
	})
	return nil
}

// turnExecution carries everything the generation loop needs once the turn
// has been assembled from the payload.
type turnExecution struct {
	processCtx  context.Context
	persistCtx  context.Context
	job         RunJobPayload
	session     domain.Session
	attachments []domain.Attachment
	startedAt   time.Time
	active      *streamingsvc.ActiveRun
}

// executeTurn is the generation loop extracted from the former
// chatServiceImpl.runStreamLoop. It never returns an error: every failure is
// broadcast as a terminal chunk and reflected into the run state, so callers
// (inline handler, worker) observe the outcome through the run's terminal
// event rather than a returned error. Only assembly failures in Execute
// propagate as errors — those are the transient infrastructure faults a
// worker may retry.
func (e *chatRunExecutor) executeTurn(t turnExecution) {
	svc := e.svc
	job := t.job
	session := t.session
	runID := job.RunID
	active := t.active
	const op serrors.Op = "chatRunExecutor.executeTurn"

	// Cleanup order matters: cancel first so generator work stops before closing
	// subscriber channels and unregistering/removing run bookkeeping.
	defer func() {
		if active.Cancel != nil {
			active.Cancel()
		}
		active.CloseAllSubscribers()
		svc.runRegistry.Remove(active.RunID)
		svc.unregisterStreamCancel(job.SessionID)
		// Shorten the run-events stream TTL now that the run is done; the
		// reaper doesn't need it any more and long-lived Redis keys for
		// every historical run would bloat memory unnecessarily. 5 min
		// gives slow reconnecting clients a small grace window.
		if svc.eventLog != nil {
			_ = svc.eventLog.DropAfterTerminal(t.persistCtx, session.TenantID(), runID, 5*time.Minute)
		}
		// The run ahead of the session FIFO terminated — hand the stream to
		// the next queued payload, if any.
		svc.promoteNextQueuedRun(t.persistCtx, session.TenantID(), job.SessionID)
	}()

	gen, err := svc.agentService.ProcessMessage(t.processCtx, job.SessionID, job.Content, t.attachments)
	if err != nil {
		active.Broadcast(streamingsvc.TerminalChunk(err, 0))
		_ = svc.cancelRunState(t.persistCtx, session.TenantID(), job.SessionID, runID)
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
		event, err := gen.Next(t.processCtx)
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
			// Track the UTF-16 code unit count incrementally so the
			// text_block_end boundary path is O(delta) instead of
			// O(total_content).
			active.ContentUTF16Len += len(utf16.Encode([]rune(event.Content)))
			active.Mu.Unlock()
			chunk.Type = bichatservices.ChunkTypeContent
			chunk.Content = event.Content
			active.Broadcast(chunk)

		case agents.EventTypeTextBlockEnd:
			active.Mu.Lock()
			// Record the running UTF-16 code unit count at the segment
			// boundary so resume snapshots can split the accumulated content
			// back into the blocks the user originally saw.
			active.TextBlockOffsets = append(active.TextBlockOffsets, active.ContentUTF16Len)
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
			generationMs = time.Since(t.startedAt).Milliseconds()
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
			_ = svc.updateRunSnapshot(t.persistCtx, session.TenantID(), job.SessionID, runID, content, meta)
			// Refresh heartbeat at the snapshot throttle cadence (2s).
			// The reaper marks runs whose heartbeat is older than ~60s as
			// failed, so a 2s cadence leaves comfortable headroom under
			// slow LLM calls or tool executions.
			_ = svc.runState.Heartbeat(t.persistCtx, session.TenantID(), job.SessionID, runID)
			// Check the out-of-band cancel flag. A Stop RPC from another
			// tab / device sets this flag on the persisted run; we
			// observe it here and wind the generator down by cancelling
			// processCtx. The next gen.Next will return ctx.Err() and
			// the outer loop's cleanup will emit a terminal chunk.
			if persistedRun, err := svc.runState.GetPersistedRun(t.persistCtx, job.SessionID); err == nil && persistedRun != nil {
				if persistedRun.CancelRequested() && active.Cancel != nil {
					active.Cancel()
				}
			}
			active.Mu.Lock()
			active.LastPersist = time.Now()
			active.Mu.Unlock()
		}
	}

	if t.processCtx.Err() != nil {
		svc.log().
			WithError(serrors.E(op, t.processCtx.Err())).
			WithField("session_id", job.SessionID.String()).
			WithField("run_id", runID.String()).
			WithField("tenant_id", session.TenantID().String()).
			Error("bichat: stream generation context ended before finalization")
		active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, t.processCtx.Err()), 0))
		_ = svc.cancelRunState(t.persistCtx, session.TenantID(), job.SessionID, runID)
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
		job.SessionID,
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
		job.Content,
		assistantContent,
		t.startedAt,
		svc.langfuseBaseURL,
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
		SessionID:    job.SessionID,
		Content:      assistantContent,
		ToolCalls:    savedToolCalls,
		DebugTrace:   assistantDebugTrace,
		QuestionData: assistantQuestionData,
	})
	if err != nil {
		svc.log().
			WithError(serrors.E(op, serrors.KindValidation, err)).
			WithField("session_id", job.SessionID.String()).
			WithField("run_id", runID.String()).
			WithField("tenant_id", session.TenantID().String()).
			WithField("content_len", len(assistantContent)).
			WithField("tool_calls", len(savedToolCalls)).
			Error("bichat: assistant message failed validation before persistence")
		active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, serrors.KindValidation, err), 0))
		_ = svc.cancelRunState(t.persistCtx, session.TenantID(), job.SessionID, runID)
		return
	}

	// Persistence is split into a small, retried CRITICAL transaction (the
	// assistant message + the session's previous-response pointer) and a
	// BEST-EFFORT artifact transaction. The rendered answer is the
	// irreplaceable artifact, so it must survive even when artifact writes or a
	// transient DB hiccup fail — the previous all-or-nothing transaction under a
	// tight deadline discarded fully-generated answers (see #2998).
	session = session.SetPreviousResponseID(providerResponseID, time.Now())
	if err := svc.persistAssistantMessageCritical(t.persistCtx, assistantMsg, session); err != nil {
		svc.log().
			WithError(err).
			WithField("session_id", job.SessionID.String()).
			WithField("run_id", runID.String()).
			WithField("tenant_id", session.TenantID().String()).
			WithField("content_len", len(assistantContent)).
			WithField("tool_calls", len(savedToolCalls)).
			WithField("artifact_count", len(artifactMap)).
			WithField("generation_ms", generationMs).
			Error("bichat: failed to persist assistant message; answer discarded")
		active.Broadcast(streamingsvc.TerminalChunk(err, 0))
		runStateCtx, runStateCancel := context.WithTimeout(context.WithoutCancel(t.persistCtx), streamPersistenceTimeout)
		defer runStateCancel()
		_ = svc.cancelRunState(runStateCtx, session.TenantID(), job.SessionID, runID)
		return
	}

	// Best-effort: the message is already committed and is what the user sees.
	// A failure here degrades to "answer without its charts/tables" rather than
	// discarding the whole answer. Artifacts carry idempotency keys, so a later
	// regeneration won't duplicate them.
	if len(artifactMap) > 0 {
		if err := svc.persistArtifactsBestEffort(t.persistCtx, session, assistantMsg.ID(), artifactMap); err != nil {
			svc.log().
				WithError(err).
				WithField("session_id", job.SessionID.String()).
				WithField("run_id", runID.String()).
				WithField("message_id", assistantMsg.ID().String()).
				WithField("artifact_count", len(artifactMap)).
				Error("bichat: failed to persist generated artifacts; answer kept without them")
		}
	}

	runStateCtx, runStateCancel := context.WithTimeout(context.WithoutCancel(t.persistCtx), streamPersistenceTimeout)
	defer runStateCancel()
	_ = svc.completeRunState(runStateCtx, session.TenantID(), job.SessionID, runID)
	if emitDoneChunk || interrupt != nil {
		active.Broadcast(streamingsvc.TerminalChunk(nil, generationMs))
	}
	if interrupt == nil {
		svc.maybeGenerateTitleAsync(t.persistCtx, job.SessionID)
	}
}

// saveAttachmentArtifacts persists one attachment artifact per domain
// attachment, keyed idempotently by message + file name.
func (s *chatServiceImpl) saveAttachmentArtifacts(ctx context.Context, session domain.Session, messageID uuid.UUID, attachments []domain.Attachment) error {
	const op serrors.Op = "chatServiceImpl.saveAttachmentArtifacts"

	for _, att := range attachments {
		artifact := domain.ArtifactSpec{
			TenantID:       session.TenantID(),
			SessionID:      session.ID(),
			MessageID:      &messageID,
			Type:           domain.ArtifactTypeAttachment,
			Name:           att.FileName(),
			MimeType:       att.MimeType(),
			URL:            att.FilePath(),
			SizeBytes:      att.SizeBytes(),
			Status:         domain.ArtifactStatusAvailable,
			IdempotencyKey: "attachment:" + messageID.String() + ":" + att.FileName(),
		}
		if att.UploadID() != nil {
			artifact.UploadID = att.UploadID()
		}
		artifactEntity, err := domain.NewArtifactFromSpec(artifact)
		if err != nil {
			return serrors.E(op, serrors.KindValidation, err)
		}
		if err := s.chatRepo.SaveArtifact(ctx, artifactEntity); err != nil {
			return serrors.E(op, err)
		}
	}
	return nil
}

// promoteNextQueuedRun pops the head of the session FIFO (enqueue mode) and
// re-enqueues it onto the run job stream so a waiting worker picks it up.
// Safe to call when the session queue or job queue is unconfigured (no-op).
func (s *chatServiceImpl) promoteNextQueuedRun(ctx context.Context, tenantID, sessionID uuid.UUID) {
	if s.runSessionQueue == nil || s.runJobQueue == nil {
		return
	}
	promoted, err := PromoteNextQueuedRun(ctx, s.runSessionQueue, s.runJobQueue, s.activeRunIndex, tenantID, sessionID)
	if err != nil {
		if e := s.logEntry(); e != nil {
			e.WithError(err).WithFields(logrus.Fields{
				"tenant_id":  tenantID.String(),
				"session_id": sessionID.String(),
			}).Error("bichat: failed to promote next queued run")
		}
		return
	}
	if promoted {
		if e := s.logEntry(); e != nil {
			e.WithFields(logrus.Fields{
				"tenant_id":  tenantID.String(),
				"session_id": sessionID.String(),
			}).Info("bichat: promoted queued run after terminal transition")
		}
	}
}

// PromoteNextQueuedRun drains one job from the per-session FIFO and posts it
// onto the run job stream. Returns false when the queue is empty. Used by the
// executor's terminal path, the run worker's terminal-failure path, and the
// reaper's OnRunTerminal hook. A failed stream enqueue pushes the job back to
// the head of the FIFO so it is never lost; the index's queued count is
// decremented after a successful promotion (nil-safe).
func PromoteNextQueuedRun(ctx context.Context, sessionQueue *RedisRunSessionQueue, jobQueue *RedisRunJobQueue, index ActiveRunIndex, tenantID, sessionID uuid.UUID) (bool, error) {
	if sessionQueue == nil || jobQueue == nil {
		return false, nil
	}
	queued, ok, err := sessionQueue.Pop(ctx, tenantID, sessionID)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	queued.Payload.Attempt = 0
	queued.Payload.EnqueuedAt = time.Now().UTC()
	if _, err := jobQueue.EnqueueClaimed(ctx, queued.Payload); err != nil {
		// Restore the job to the head of the FIFO so it keeps its position
		// — the client is already tailing this run's event log, so losing
		// the job here would strand that stream without a terminal event.
		_ = sessionQueue.PushFront(ctx, tenantID, sessionID, queued)
		return false, err
	}
	if index != nil {
		_ = index.AddQueuedRuns(ctx, tenantID, sessionID, queued.Payload.RunID, -1)
	}
	return true, nil
}

// resolveUploadAttachments maps upload ids back to domain attachments so a
// worker process can rebuild the attachment list from the payload without
// access to the original request objects.
func resolveUploadAttachments(ctx context.Context, uploadIDs []int64) ([]domain.Attachment, error) {
	const op serrors.Op = "resolveUploadAttachments"

	if len(uploadIDs) == 0 {
		return nil, nil
	}

	unique := make([]uint, 0, len(uploadIDs))
	seen := make(map[int64]struct{}, len(uploadIDs))
	for _, id := range uploadIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, uint(id))
	}

	repo := corepersistence.NewUploadRepository()
	found, err := repo.GetByIDs(ctx, unique)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	byID := make(map[uint]int, len(found))
	for i, entity := range found {
		byID[entity.ID()] = i
	}

	result := make([]domain.Attachment, 0, len(uploadIDs))
	for _, id := range uploadIDs {
		idx, ok := byID[uint(id)]
		if !ok {
			return nil, serrors.E(op, serrors.KindValidation, fmt.Errorf("upload not found: %d", id))
		}
		entity := found[idx]
		mimeType := ""
		if entity.Mimetype() != nil {
			mimeType = entity.Mimetype().String()
		}
		result = append(result, domain.NewAttachment(
			domain.WithUploadID(int64(entity.ID())),
			domain.WithFileName(entity.Name()),
			domain.WithMimeType(mimeType),
			domain.WithSizeBytes(int64(entity.Size().Bytes())),
			domain.WithFilePath(entity.URL().String()),
		))
	}
	return result, nil
}
