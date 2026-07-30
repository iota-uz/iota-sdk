package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ContinueSession starts a trusted internal turn in an existing session.
func (s *chatServiceImpl) ContinueSession(
	ctx context.Context,
	req bichatservices.ContinueSessionRequest,
) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.ContinueSession"

	if req.SessionID == uuid.Nil || strings.TrimSpace(req.IdempotencyKey) == "" {
		return bichatservices.AsyncRunAccepted{}, serrors.E(op, serrors.KindValidation, bichatservices.ErrInvalidContinuation)
	}
	if err := req.Event.Validate(); err != nil {
		return bichatservices.AsyncRunAccepted{}, serrors.E(op, serrors.KindValidation, err)
	}

	continuationAgent, ok := s.agentService.(bichatservices.ContinuationAgentService)
	if !ok {
		return bichatservices.AsyncRunAccepted{}, serrors.E(op, bichatservices.ErrContinuationUnsupported)
	}

	return s.startAsyncRun(
		ctx,
		req.SessionID,
		bichatservices.AsyncRunOperationContinuation,
		req.IdempotencyKey,
		func(txCtx context.Context, _ domain.Session) error {
			return s.ensureNoOpenQuestionForSend(txCtx, req.SessionID)
		},
		func(
			processCtx context.Context,
			persistCtx context.Context,
			runID uuid.UUID,
			session domain.Session,
			active *streamingsvc.ActiveRun,
		) {
			defer func() {
				if active.Cancel != nil {
					active.Cancel()
				}
				active.CloseAllSubscribers()
				s.runRegistry.Remove(active.RunID)
				s.unregisterStreamCancel(req.SessionID)
				if s.eventLog != nil {
					_ = s.eventLog.DropAfterTerminal(persistCtx, session.TenantID(), runID, 5*time.Minute)
				}
			}()

			if req.ReasoningEffort != nil {
				processCtx = bichatservices.WithReasoningEffort(processCtx, *req.ReasoningEffort)
			}
			if req.Model != nil {
				processCtx = bichatservices.WithModelOverride(processCtx, *req.Model)
			}
			if s.eventLog != nil {
				active.SetMirror(func(chunk bichatservices.StreamChunk) {
					eventType, body, err := encodeRunEventFromChunk(chunk)
					if err != nil {
						return
					}
					_, _ = s.eventLog.Append(persistCtx, session.TenantID(), runID, RunEvent{
						Type:    eventType,
						Payload: body,
					})
				})
			}

			startedAt := time.Now()
			gen, err := continuationAgent.ProcessContinuation(processCtx, req.SessionID, req.Event)
			if err != nil {
				s.failContinuationRun(persistCtx, active, op, err, session, runID)
				return
			}
			defer gen.Close()

			result, err := consumeAgentEvents(processCtx, gen)
			if err != nil {
				s.failContinuationRun(persistCtx, active, op, err, session, runID)
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
				map[string]any{
					"tool_calls":      result.toolCalls,
					"continuation":    true,
					"correlation_id":  req.Event.CorrelationID,
					"idempotency_key": req.IdempotencyKey,
				},
			)

			if err := processCtx.Err(); err != nil {
				s.failContinuationRun(persistCtx, active, op, err, session, runID)
				return
			}

			saveCtx, cancel := context.WithTimeout(persistCtx, streamPersistenceTimeout)
			defer cancel()
			eventPrompt, _ := req.Event.Prompt()
			err = s.withinTx(saveCtx, func(txCtx context.Context) error {
				_, _, saveErr := s.saveAgentResult(
					txCtx,
					op,
					session,
					req.SessionID,
					result,
					startedAt,
					eventPrompt,
				)
				return saveErr
			})
			if err != nil {
				s.failContinuationRun(persistCtx, active, op, err, session, runID)
				return
			}
			if err := s.completeRunState(persistCtx, session.TenantID(), req.SessionID, runID); err != nil {
				s.failContinuationRun(persistCtx, active, op, err, session, runID)
				return
			}
			active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
		},
	)
}

func (s *chatServiceImpl) failContinuationRun(
	ctx context.Context,
	active *streamingsvc.ActiveRun,
	op serrors.Op,
	err error,
	session domain.Session,
	runID uuid.UUID,
) {
	active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
	_ = s.withinTx(context.WithoutCancel(ctx), func(txCtx context.Context) error {
		return s.chatRepo.CancelRun(txCtx, runID)
	})
	if failErr := s.runState.FailRunState(
		context.WithValue(ctx, constants.TxKey, nil),
		session.TenantID(),
		session.ID(),
		runID,
	); failErr != nil && !errors.Is(failErr, domain.ErrRunNotFound) {
		_ = s.cancelRunState(ctx, session.TenantID(), session.ID(), runID)
		return
	}
	s.publishTerminalStatus(ctx, session.TenantID(), session.ID(), runID, string(domain.GenerationRunStatusFailed))
}
