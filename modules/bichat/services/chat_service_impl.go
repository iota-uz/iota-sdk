package services

import (
	"context"
	"errors"
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
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const streamPersistenceTimeout = 10 * time.Second
const titleGenerationFallbackTimeout = 15 * time.Second
const streamSnapshotThrottle = 2 * time.Second
const sessionAccessRepoNotConfiguredError = "session access repository is not configured"

// chatServiceImpl is the production implementation of ChatService.
// It orchestrates chat sessions, messages, and agent execution.
type chatServiceImpl struct {
	chatRepo           domain.ChatRepository
	agentService       bichatservices.AgentService
	model              agents.Model
	titleService       TitleService
	titleQueue         TitleJobQueue
	runState           *streamingsvc.RunStateManager
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
	return &chatServiceImpl{
		chatRepo:           chatRepo,
		agentService:       agentService,
		model:              model,
		titleService:       titleService,
		titleQueue:         titleQueue,
		runState:           streamingsvc.NewRunStateManager(runStore),
		activeStreamCancel: make(map[uuid.UUID]context.CancelFunc),
		runRegistry:        streamingsvc.NewRunRegistry(),
	}
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

// StopGeneration cancels the active stream for the session; no partial assistant message is persisted.
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
	return s.runState.CreateRunState(ctx, run)
}

func (s *chatServiceImpl) getPersistedRun(ctx context.Context, sessionID uuid.UUID) (domain.GenerationRun, error) {
	return s.runState.GetPersistedRun(ctx, sessionID)
}

func (s *chatServiceImpl) updateRunSnapshot(ctx context.Context, tenantID, sessionID, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	return s.runState.UpdateRunSnapshot(ctx, tenantID, sessionID, runID, partialContent, partialMetadata)
}

func (s *chatServiceImpl) completeRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.runState.CompleteRunState(ctx, tenantID, sessionID, runID)
}

func (s *chatServiceImpl) cancelRunState(ctx context.Context, tenantID, sessionID, runID uuid.UUID) error {
	return s.runState.CancelRunState(ctx, tenantID, sessionID, runID)
}

// ResumeStream attaches to an active run and streams snapshot then new chunks.
func (s *chatServiceImpl) ResumeStream(ctx context.Context, sessionID uuid.UUID, runID uuid.UUID, onChunk func(bichatservices.StreamChunk)) error {
	const op serrors.Op = "chatServiceImpl.ResumeStream"

	run := s.runRegistry.GetByRun(runID)
	if run == nil {
		return bichatservices.ErrRunNotFoundOrFinished
	}
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

// SendMessage sends a message to a session and processes it with the agent.
func (s *chatServiceImpl) SendMessage(ctx context.Context, req bichatservices.SendMessageRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.SendMessage"
	startedAt := time.Now()

	var session domain.Session
	var err error

	// Create user message (omit author when UserID is 0 so SaveMessage can fall back to session owner)
	userMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	if req.UserID != 0 {
		userMsgOpts = append(userMsgOpts, types.WithAuthorUserID(req.UserID))
	}
	userMsg := types.UserMessage(req.Content, userMsgOpts...)

	processCtx := bichatservices.WithArtifactMessageID(ctx, userMsg.ID())

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

		if err := s.chatRepo.SaveMessage(txCtx, userMsg); err != nil {
			return serrors.E(op, err)
		}

		for _, att := range domainAttachments {
			msgID := userMsg.ID()
			artifactOpts := []domain.ArtifactOption{
				domain.WithArtifactTenantID(session.TenantID()),
				domain.WithArtifactSessionID(session.ID()),
				domain.WithArtifactMessageID(&msgID),
				domain.WithArtifactType(domain.ArtifactTypeAttachment),
				domain.WithArtifactName(att.FileName()),
				domain.WithArtifactMimeType(att.MimeType()),
				domain.WithArtifactURL(att.FilePath()),
				domain.WithArtifactSizeBytes(att.SizeBytes()),
			}
			if att.UploadID() != nil {
				artifactOpts = append(artifactOpts, domain.WithArtifactUploadID(*att.UploadID()))
			}
			artifact := domain.NewArtifact(artifactOpts...)
			if err := s.chatRepo.SaveArtifact(txCtx, artifact); err != nil {
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

	var session domain.Session
	var err error

	// Create user message (omit author when UserID is 0 so SaveMessage can fall back to session owner)
	userMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	if req.UserID != 0 {
		userMsgOpts = append(userMsgOpts, types.WithAuthorUserID(req.UserID))
	}
	userMsg := types.UserMessage(req.Content, userMsgOpts...)

	domainAttachments := cloneAttachmentsForMessage(userMsg.ID(), req.Attachments)

	var run domain.GenerationRun
	runStateCreated := false

	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err = s.chatRepo.GetSession(txCtx, req.SessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		run = domain.NewGenerationRun(
			domain.WithGenerationRunSessionID(req.SessionID),
			domain.WithGenerationRunTenantID(session.TenantID()),
			domain.WithGenerationRunUserID(session.UserID()),
		)
		runStateCreated, err = s.createRunState(txCtx, run)
		if err != nil {
			return err
		}

		session, err = s.maybeReplaceHistoryFromMessage(txCtx, session, req.ReplaceFromMessageID)
		if err != nil {
			return serrors.E(op, err)
		}

		if err := s.chatRepo.SaveMessage(txCtx, userMsg); err != nil {
			return serrors.E(op, err)
		}

		for _, att := range domainAttachments {
			msgID := userMsg.ID()
			artifactOpts := []domain.ArtifactOption{
				domain.WithArtifactTenantID(session.TenantID()),
				domain.WithArtifactSessionID(session.ID()),
				domain.WithArtifactMessageID(&msgID),
				domain.WithArtifactType(domain.ArtifactTypeAttachment),
				domain.WithArtifactName(att.FileName()),
				domain.WithArtifactMimeType(att.MimeType()),
				domain.WithArtifactURL(att.FilePath()),
				domain.WithArtifactSizeBytes(att.SizeBytes()),
			}
			if att.UploadID() != nil {
				artifactOpts = append(artifactOpts, domain.WithArtifactUploadID(*att.UploadID()))
			}
			artifact := domain.NewArtifact(artifactOpts...)
			if err := s.chatRepo.SaveArtifact(txCtx, artifact); err != nil {
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

	active := streamingsvc.NewActiveRun(run.ID(), req.SessionID, cancelProcess, time.Now())
	primaryCh := make(chan bichatservices.StreamChunk, 256)
	active.AddSubscriber(primaryCh)
	s.runRegistry.Add(active)

	persistCtx := context.WithoutCancel(ctx)
	// Stream finalization may outlive request-scoped middleware transactions.
	// Clear TxKey so persistence always opens its own durable transaction.
	persistCtx = context.WithValue(persistCtx, constants.TxKey, nil)

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
	assistantMsgOpts := []types.MessageOption{types.WithSessionID(req.SessionID)}
	if len(savedToolCalls) > 0 {
		assistantMsgOpts = append(assistantMsgOpts, types.WithToolCalls(savedToolCalls...))
	}
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
		assistantMsgOpts = append(assistantMsgOpts, types.WithDebugTrace(debugTrace))
	}
	if interrupt != nil {
		qd, err := hitlsvc.BuildQuestionData(interrupt.CheckpointID, interruptAgentName, interrupt.Questions)
		if err == nil && qd != nil {
			assistantMsgOpts = append(assistantMsgOpts, types.WithQuestionData(qd))
		}
	}

	assistantMsg := types.AssistantMessage(assistantContent, assistantMsgOpts...)
	persistCtx, persistCancel := context.WithTimeout(persistCtx, streamPersistenceTimeout)
	defer persistCancel()

	err = s.withinTx(persistCtx, func(txCtx context.Context) error {
		if err := s.chatRepo.SaveMessage(txCtx, assistantMsg); err != nil {
			return serrors.E(op, err)
		}
		if err := s.persistGeneratedArtifacts(txCtx, session, assistantMsg.ID(), artifactMap); err != nil {
			return serrors.E(op, err)
		}
		session = session.
			UpdateLLMPreviousResponseID(providerResponseID).
			UpdateUpdatedAt(time.Now())
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
