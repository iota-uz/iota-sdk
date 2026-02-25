package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	hitlsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/hitl"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

func (s *chatServiceImpl) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	const op serrors.Op = "chatServiceImpl.GetSessionMessages"

	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return messages, nil
}

// ResumeWithAnswer resumes execution after user answers questions (HITL).
func (s *chatServiceImpl) ResumeWithAnswer(ctx context.Context, req bichatservices.ResumeRequest) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.ResumeWithAnswer"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Get pending question message
	pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoPendingQuestion) {
			return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
		}
		return nil, serrors.E(op, err)
	}
	if pendingMsg == nil {
		return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
	}

	// Validate question data before resuming (defer mutation until resume succeeds)
	qd := pendingMsg.QuestionData()
	if qd == nil {
		return nil, serrors.E(op, serrors.KindValidation, "pending message has no question data")
	}

	canonicalCheckpointID := strings.TrimSpace(qd.CheckpointID)
	if canonicalCheckpointID == "" {
		return nil, serrors.E(op, serrors.KindValidation, "pending message has empty checkpoint id")
	}
	resolvedCheckpointID, checkpointMismatch := hitlsvc.ResolveCheckpoint(req.CheckpointID, canonicalCheckpointID)
	if checkpointMismatch {
		configuration.Use().Logger().
			WithField("session_id", req.SessionID.String()).
			WithField("requested_checkpoint_id", strings.TrimSpace(req.CheckpointID)).
			WithField("canonical_checkpoint_id", canonicalCheckpointID).
			Warn("resume request checkpoint mismatch; using canonical checkpoint from pending question")
	}

	normalizedAnswerValues, answersMap, err := hitlsvc.NormalizeAnswers(qd.Questions, req.Answers)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
	answeredQD, err := qd.Answer(normalizedAnswerValues)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Resume agent execution with answers
	gen, err := s.agentService.ResumeWithAnswer(ctx, req.SessionID, resolvedCheckpointID, answersMap)
	if err != nil {
		if errors.Is(err, agents.ErrCheckpointNotFound) {
			configuration.Use().Logger().
				WithError(err).
				WithField("session_id", req.SessionID.String()).
				WithField("checkpoint_id", resolvedCheckpointID).
				Warn("resume checkpoint missing; finalizing pending question as answered")

			if txErr := s.withinTx(ctx, func(txCtx context.Context) error {
				return s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsg.ID(), answeredQD)
			}); txErr != nil {
				return nil, serrors.E(op, txErr)
			}

			return &bichatservices.SendMessageResponse{
				UserMessage:      nil,
				AssistantMessage: nil,
				Session:          session,
				Interrupt:        nil,
			}, nil
		}
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(ctx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var assistantMsg types.Message
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		// Mark question as answered only after resume succeeds — prevents irreversible
		// state drift if the provider returns a transient error, timeout, or bad checkpoint.
		if err := s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsg.ID(), answeredQD); err != nil {
			return serrors.E(op, err)
		}

		assistantMsg, session, err = s.saveAgentResult(txCtx, op, session, req.SessionID, result, startedAt, "")
		return err
	})
	if err != nil {
		return nil, err
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      nil,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// ResumeWithAnswerAsync starts HITL resume as an async run and returns run metadata.
func (s *chatServiceImpl) ResumeWithAnswerAsync(ctx context.Context, req bichatservices.ResumeRequest) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.ResumeWithAnswerAsync"

	var (
		pendingMsgID         uuid.UUID
		resolvedCheckpointID string
		answersMap           map[string]types.Answer
		answeredQuestionData *types.QuestionData
	)
	return s.startAsyncRun(
		ctx,
		req.SessionID,
		bichatservices.AsyncRunOperationQuestionSubmit,
		func(txCtx context.Context, _ domain.Session) error {
			pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(txCtx, req.SessionID)
			if err != nil {
				if errors.Is(err, domain.ErrNoPendingQuestion) {
					return serrors.E(op, serrors.KindValidation, "no pending question found for session")
				}
				return serrors.E(op, err)
			}
			if pendingMsg == nil {
				return serrors.E(op, serrors.KindValidation, "no pending question found for session")
			}

			qd := pendingMsg.QuestionData()
			if qd == nil {
				return serrors.E(op, serrors.KindValidation, "pending message has no question data")
			}
			canonicalCheckpointID := strings.TrimSpace(qd.CheckpointID)
			if canonicalCheckpointID == "" {
				return serrors.E(op, serrors.KindValidation, "pending message has empty checkpoint id")
			}
			resolvedCheckpointID, _ = hitlsvc.ResolveCheckpoint(req.CheckpointID, canonicalCheckpointID)

			normalizedAnswerValues, normalizedAnswersMap, err := hitlsvc.NormalizeAnswers(qd.Questions, req.Answers)
			if err != nil {
				return serrors.E(op, serrors.KindValidation, err)
			}
			answeredQuestionData, err = qd.Answer(normalizedAnswerValues)
			if err != nil {
				return serrors.E(op, err)
			}
			pendingMsgID = pendingMsg.ID()
			answersMap = normalizedAnswersMap
			return nil
		},
		func(processCtx context.Context, persistCtx context.Context, runID uuid.UUID, session domain.Session, active *streamingsvc.ActiveRun) {
			defer func() {
				if active.Cancel != nil {
					active.Cancel()
				}
				active.CloseAllSubscribers()
				s.runRegistry.Remove(active.RunID)
				s.unregisterStreamCancel(req.SessionID)
			}()

			startedAt := time.Now()
			gen, err := s.agentService.ResumeWithAnswer(processCtx, req.SessionID, resolvedCheckpointID, answersMap)
			if err != nil {
				if errors.Is(err, agents.ErrCheckpointNotFound) {
					if processErr := processCtx.Err(); processErr != nil {
						active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processErr), 0))
						_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
						return
					}
					updateCtx, cancel := context.WithTimeout(processCtx, streamPersistenceTimeout)
					defer cancel()
					if txErr := s.withinTx(updateCtx, func(txCtx context.Context) error {
						return s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsgID, answeredQuestionData)
					}); txErr != nil {
						active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, txErr), 0))
						_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
						return
					}
					_ = s.completeRunState(persistCtx, session.TenantID(), req.SessionID, runID)
					active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
					return
				}
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
				return
			}
			defer gen.Close()

			result, err := consumeAgentEvents(processCtx, gen)
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
				return
			}

			if strings.TrimSpace(result.content) != "" {
				active.Mu.Lock()
				active.Content = result.content
				active.Mu.Unlock()
				active.Broadcast(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeContent,
					Content:   result.content,
					Timestamp: time.Now(),
				})
			}

			_ = s.updateRunSnapshot(
				persistCtx,
				session.TenantID(),
				req.SessionID,
				runID,
				result.content,
				map[string]any{"tool_calls": result.toolCalls},
			)

			if processErr := processCtx.Err(); processErr != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processErr), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
				return
			}
			persistRunCtx, persistCancel := context.WithTimeout(processCtx, streamPersistenceTimeout)
			defer persistCancel()
			err = s.withinTx(persistRunCtx, func(txCtx context.Context) error {
				if err := s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsgID, answeredQuestionData); err != nil {
					return serrors.E(op, err)
				}
				_, _, saveErr := s.saveAgentResult(txCtx, op, session, req.SessionID, result, startedAt, "")
				return saveErr
			})
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(err, 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), req.SessionID, runID)
				return
			}

			_ = s.completeRunState(persistCtx, session.TenantID(), req.SessionID, runID)
			active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
		},
	)
}

// RejectPendingQuestion rejects a pending HITL question and resumes execution.
// This marks the question data as rejected and tells the agent the user dismissed it.
func (s *chatServiceImpl) RejectPendingQuestion(ctx context.Context, sessionID uuid.UUID) (*bichatservices.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceImpl.RejectPendingQuestion"
	startedAt := time.Now()

	// Get session
	session, err := s.chatRepo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Get pending question message
	pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(ctx, sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNoPendingQuestion) {
			return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
		}
		return nil, serrors.E(op, err)
	}
	if pendingMsg == nil {
		return nil, serrors.E(op, serrors.KindValidation, "no pending question found for session")
	}

	// Validate question data before resuming (defer mutation until resume succeeds)
	qd := pendingMsg.QuestionData()
	if qd == nil {
		return nil, serrors.E(op, serrors.KindValidation, "pending message has no question data")
	}

	rejectedQD, err := qd.Reject()
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Resume agent with rejection signal
	rejectionAnswers := map[string]types.Answer{
		"__rejected__": types.NewAnswer("User dismissed the questions"),
	}

	gen, err := s.agentService.ResumeWithAnswer(ctx, sessionID, qd.CheckpointID, rejectionAnswers)
	if err != nil {
		if errors.Is(err, agents.ErrCheckpointNotFound) {
			configuration.Use().Logger().
				WithError(err).
				WithField("session_id", sessionID.String()).
				WithField("checkpoint_id", qd.CheckpointID).
				Warn("reject checkpoint missing; finalizing pending question as rejected")

			if txErr := s.withinTx(ctx, func(txCtx context.Context) error {
				return s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsg.ID(), rejectedQD)
			}); txErr != nil {
				return nil, serrors.E(op, txErr)
			}

			return &bichatservices.SendMessageResponse{
				UserMessage:      nil,
				AssistantMessage: nil,
				Session:          session,
				Interrupt:        nil,
			}, nil
		}
		return nil, serrors.E(op, err)
	}
	defer gen.Close()

	// Collect agent response
	result, err := consumeAgentEvents(ctx, gen)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	var assistantMsg types.Message
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		// Mark question as rejected only after resume succeeds — prevents irreversible
		// state drift if the provider returns a transient error, timeout, or bad checkpoint.
		if err := s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsg.ID(), rejectedQD); err != nil {
			return serrors.E(op, err)
		}

		assistantMsg, session, err = s.saveAgentResult(txCtx, op, session, sessionID, result, startedAt, "")
		return err
	})
	if err != nil {
		return nil, err
	}

	return &bichatservices.SendMessageResponse{
		UserMessage:      nil,
		AssistantMessage: assistantMsg,
		Session:          session,
		Interrupt:        result.interrupt,
	}, nil
}

// RejectPendingQuestionAsync starts question rejection as an async run and returns run metadata.
func (s *chatServiceImpl) RejectPendingQuestionAsync(ctx context.Context, sessionID uuid.UUID) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.RejectPendingQuestionAsync"

	var (
		pendingMsgID         uuid.UUID
		checkpointID         string
		rejectedQuestionData *types.QuestionData
		rejectionAnswers     map[string]types.Answer
	)
	rejectionAnswers = map[string]types.Answer{
		"__rejected__": types.NewAnswer("User dismissed the questions"),
	}

	return s.startAsyncRun(
		ctx,
		sessionID,
		bichatservices.AsyncRunOperationQuestionReject,
		func(txCtx context.Context, _ domain.Session) error {
			pendingMsg, err := s.chatRepo.GetPendingQuestionMessage(txCtx, sessionID)
			if err != nil {
				if errors.Is(err, domain.ErrNoPendingQuestion) {
					return serrors.E(op, serrors.KindValidation, "no pending question found for session")
				}
				return serrors.E(op, err)
			}
			if pendingMsg == nil {
				return serrors.E(op, serrors.KindValidation, "no pending question found for session")
			}

			qd := pendingMsg.QuestionData()
			if qd == nil {
				return serrors.E(op, serrors.KindValidation, "pending message has no question data")
			}

			rejectedQuestionData, err = qd.Reject()
			if err != nil {
				return serrors.E(op, err)
			}
			pendingMsgID = pendingMsg.ID()
			checkpointID = qd.CheckpointID
			return nil
		},
		func(processCtx context.Context, persistCtx context.Context, runID uuid.UUID, session domain.Session, active *streamingsvc.ActiveRun) {
			defer func() {
				if active.Cancel != nil {
					active.Cancel()
				}
				active.CloseAllSubscribers()
				s.runRegistry.Remove(active.RunID)
				s.unregisterStreamCancel(sessionID)
			}()

			startedAt := time.Now()
			gen, err := s.agentService.ResumeWithAnswer(processCtx, sessionID, checkpointID, rejectionAnswers)
			if err != nil {
				if errors.Is(err, agents.ErrCheckpointNotFound) {
					if processErr := processCtx.Err(); processErr != nil {
						active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processErr), 0))
						_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
						return
					}
					updateCtx, cancel := context.WithTimeout(processCtx, streamPersistenceTimeout)
					defer cancel()
					if txErr := s.withinTx(updateCtx, func(txCtx context.Context) error {
						return s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsgID, rejectedQuestionData)
					}); txErr != nil {
						active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, txErr), 0))
						_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
						return
					}
					_ = s.completeRunState(persistCtx, session.TenantID(), sessionID, runID)
					active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
					return
				}
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}
			defer gen.Close()

			result, err := consumeAgentEvents(processCtx, gen)
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			if strings.TrimSpace(result.content) != "" {
				active.Mu.Lock()
				active.Content = result.content
				active.Mu.Unlock()
				active.Broadcast(bichatservices.StreamChunk{
					Type:      bichatservices.ChunkTypeContent,
					Content:   result.content,
					Timestamp: time.Now(),
				})
			}
			_ = s.updateRunSnapshot(
				persistCtx,
				session.TenantID(),
				sessionID,
				runID,
				result.content,
				map[string]any{"tool_calls": result.toolCalls},
			)

			if processErr := processCtx.Err(); processErr != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processErr), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}
			persistRunCtx, persistCancel := context.WithTimeout(processCtx, streamPersistenceTimeout)
			defer persistCancel()
			err = s.withinTx(persistRunCtx, func(txCtx context.Context) error {
				if err := s.chatRepo.UpdateMessageQuestionData(txCtx, pendingMsgID, rejectedQuestionData); err != nil {
					return serrors.E(op, err)
				}
				_, _, saveErr := s.saveAgentResult(txCtx, op, session, sessionID, result, startedAt, "")
				return saveErr
			})
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(err, 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			_ = s.completeRunState(persistCtx, session.TenantID(), sessionID, runID)
			active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
		},
	)
}

// GenerateSessionTitle regenerates a session title explicitly.
func (s *chatServiceImpl) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceImpl.GenerateSessionTitle"

	if s.titleService == nil {
		return serrors.E(op, serrors.KindValidation, "title generation service is not configured")
	}

	if err := s.titleService.RegenerateSessionTitle(ctx, sessionID); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) generateCompactionSummary(ctx context.Context, messages []types.Message) (string, error) {
	if len(messages) == 0 {
		return "History compaction complete. No previous messages were available to summarize.", nil
	}

	var transcript strings.Builder
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		switch msg.Role() {
		case types.RoleUser:
			transcript.WriteString("User: ")
		case types.RoleAssistant:
			transcript.WriteString("Assistant: ")
		case types.RoleSystem, types.RoleTool:
			continue
		default:
			continue
		}
		transcript.WriteString(strings.TrimSpace(msg.Content()))
		transcript.WriteString("\n")
	}

	if transcript.Len() == 0 {
		return "History compaction complete. No user/assistant turns were available to summarize.", nil
	}

	prompt := fmt.Sprintf(
		`Summarize this chat history into a compact state snapshot.
Return markdown with these sections:
1. Conversation Summary
2. Key Facts and Decisions
3. Open Questions or Follow-ups
Keep it concise, factual, and preserve important numeric values.

CHAT TRANSCRIPT:
%s`,
		transcript.String(),
	)

	if s.model == nil {
		return "History compaction complete. A concise summary could not be generated because no model is configured.", nil
	}

	resp, err := s.model.Generate(ctx, agents.Request{
		Messages: []types.Message{types.SystemMessage(prompt)},
	}, agents.WithMaxTokens(700))
	if err != nil {
		return "History compaction complete. Summary generation failed, original history was compacted.", err
	}

	summary := strings.TrimSpace(resp.Message.Content())
	if summary == "" {
		return "History compaction complete. The model returned an empty summary.", nil
	}

	return summary, nil
}

// maybeGenerateTitleAsync enqueues durable title generation work.
// If Redis enqueue fails, it falls back to immediate direct generation.
func (s *chatServiceImpl) maybeGenerateTitleAsync(ctx context.Context, sessionID uuid.UUID) {
	// Skip if no title service configured
	if s.titleService == nil {
		return
	}

	if !isNilTitleJobQueue(s.titleQueue) {
		tenantID, tenantErr := composables.UseTenantID(ctx)
		if tenantErr == nil {
			enqueueCtx := context.WithoutCancel(ctx)
			enqueueErr := s.titleQueue.Enqueue(enqueueCtx, tenantID, sessionID)
			if enqueueErr == nil {
				return
			}
			configuration.Use().Logger().
				WithError(enqueueErr).
				WithField("session_id", sessionID.String()).
				Warn("failed to enqueue title generation job, using sync fallback")
		} else {
			configuration.Use().Logger().
				WithError(tenantErr).
				WithField("session_id", sessionID.String()).
				Warn("missing tenant context for title job enqueue, using sync fallback")
		}
	}

	titleCtx := buildTitleGenerationContext(ctx)
	titleCtx, cancel := context.WithTimeout(titleCtx, titleGenerationFallbackTimeout)
	defer cancel()

	if err := s.titleService.GenerateSessionTitle(titleCtx, sessionID); err != nil {
		configuration.Use().Logger().
			WithError(err).
			WithField("session_id", sessionID.String()).
			Warn("failed to auto-generate session title via sync fallback")
	}
}

func buildTitleGenerationContext(ctx context.Context) context.Context {
	titleCtx := context.Background()

	if tenantID, err := composables.UseTenantID(ctx); err == nil {
		titleCtx = composables.WithTenantID(titleCtx, tenantID)
	}
	if pool, err := composables.UsePool(ctx); err == nil {
		titleCtx = composables.WithPool(titleCtx, pool)
	}
	if user, err := composables.UseUser(ctx); err == nil {
		titleCtx = composables.WithUser(titleCtx, user)
	}

	return titleCtx
}

// agentToolToServiceTool converts agents.ToolEvent to bichatservices.ToolEvent
// for use in StreamChunk payloads.
func agentToolToServiceTool(t *agents.ToolEvent) *bichatservices.ToolEvent {
	if t == nil {
		return nil
	}
	return &bichatservices.ToolEvent{
		CallID:     t.CallID,
		Name:       t.Name,
		AgentName:  t.AgentName,
		Arguments:  t.Arguments,
		Result:     t.Result,
		Error:      t.Error,
		DurationMs: t.DurationMs,
		Artifacts:  t.Artifacts,
	}
}
