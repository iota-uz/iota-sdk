// Package services provides this package.
package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	streamingsvc "github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ClearSessionHistory removes all messages/artifacts while preserving session metadata.
func (s *chatServiceImpl) ClearSessionHistory(ctx context.Context, sessionID uuid.UUID) (bichatservices.ClearSessionHistoryResponse, error) {
	const op serrors.Op = "chatServiceImpl.ClearSessionHistory"

	var deletedMessages int64
	var deletedArtifacts int64
	err := s.withinTx(ctx, func(txCtx context.Context) error {
		session, err := s.chatRepo.GetSession(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		deletedMessages, err = s.chatRepo.TruncateMessagesFrom(txCtx, sessionID, time.Unix(0, 0))
		if err != nil {
			return serrors.E(op, err)
		}

		deletedArtifacts, err = s.chatRepo.DeleteSessionArtifacts(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		updated := session.SetPreviousResponseID(nil, time.Now())
		if err := s.chatRepo.UpdateSession(txCtx, updated); err != nil {
			return serrors.E(op, err)
		}
		return nil
	})
	if err != nil {
		return bichatservices.ClearSessionHistoryResponse{}, serrors.E(op, err)
	}

	return bichatservices.ClearSessionHistoryResponse{
		Success:          true,
		DeletedMessages:  deletedMessages,
		DeletedArtifacts: deletedArtifacts,
	}, nil
}

// CompactSessionHistory replaces full session history with a compacted summary turn.
func (s *chatServiceImpl) CompactSessionHistory(ctx context.Context, sessionID uuid.UUID) (bichatservices.CompactSessionHistoryResponse, error) {
	const op serrors.Op = "chatServiceImpl.CompactSessionHistory"

	messages, err := s.chatRepo.GetSessionMessages(ctx, sessionID, domain.ListOptions{
		Limit:  5000,
		Offset: 0,
	})
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	summary, err := s.generateCompactionSummary(ctx, messages)
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	var deletedMessages int64
	var deletedArtifacts int64
	err = s.withinTx(ctx, func(txCtx context.Context) error {
		session, err := s.chatRepo.GetSession(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		deletedMessages, err = s.chatRepo.TruncateMessagesFrom(txCtx, sessionID, time.Unix(0, 0))
		if err != nil {
			return serrors.E(op, err)
		}

		deletedArtifacts, err = s.chatRepo.DeleteSessionArtifacts(txCtx, sessionID)
		if err != nil {
			return serrors.E(op, err)
		}

		systemMsg := types.SystemMessage(summary, types.WithSessionID(sessionID))
		if err := s.chatRepo.SaveMessage(txCtx, systemMsg); err != nil {
			return serrors.E(op, err)
		}

		updated := session.SetPreviousResponseID(nil, time.Now())
		if err := s.chatRepo.UpdateSession(txCtx, updated); err != nil {
			return serrors.E(op, err)
		}
		return nil
	})
	if err != nil {
		return bichatservices.CompactSessionHistoryResponse{}, serrors.E(op, err)
	}

	return bichatservices.CompactSessionHistoryResponse{
		Success:          true,
		Summary:          summary,
		DeletedMessages:  deletedMessages,
		DeletedArtifacts: deletedArtifacts,
	}, nil
}

// CompactSessionHistoryAsync starts compaction as an async run and returns run metadata.
func (s *chatServiceImpl) CompactSessionHistoryAsync(ctx context.Context, sessionID uuid.UUID) (bichatservices.AsyncRunAccepted, error) {
	const op serrors.Op = "chatServiceImpl.CompactSessionHistoryAsync"

	return s.startAsyncRun(
		ctx,
		sessionID,
		bichatservices.AsyncRunOperationSessionCompact,
		nil,
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
			messages, err := s.chatRepo.GetSessionMessages(processCtx, sessionID, domain.ListOptions{
				Limit:  5000,
				Offset: 0,
			})
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			summary, err := s.generateCompactionSummary(processCtx, messages)
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			trimmed := strings.TrimSpace(summary)
			if trimmed == "" {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, serrors.KindValidation, "compaction summary is empty"), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			active.Mu.Lock()
			active.Content = trimmed
			active.Mu.Unlock()
			active.Broadcast(bichatservices.StreamChunk{
				Type:      bichatservices.ChunkTypeContent,
				Content:   trimmed,
				Timestamp: time.Now(),
			})
			if snapErr := s.updateRunSnapshot(
				persistCtx,
				session.TenantID(),
				sessionID,
				runID,
				trimmed,
				map[string]any{},
			); snapErr != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, snapErr), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			if processErr := processCtx.Err(); processErr != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, processErr), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}
			persistRunCtx, persistCancel := context.WithTimeout(persistCtx, streamPersistenceTimeout)
			defer persistCancel()

			err = s.withinTx(persistRunCtx, func(txCtx context.Context) error {
				currentSession, getErr := s.chatRepo.GetSession(txCtx, sessionID)
				if getErr != nil {
					return serrors.E(op, getErr)
				}

				_, truncateErr := s.chatRepo.TruncateMessagesFrom(txCtx, sessionID, time.Unix(0, 0))
				if truncateErr != nil {
					return serrors.E(op, truncateErr)
				}
				if _, deleteArtifactsErr := s.chatRepo.DeleteSessionArtifacts(txCtx, sessionID); deleteArtifactsErr != nil {
					return serrors.E(op, deleteArtifactsErr)
				}

				systemMsg := types.SystemMessage(trimmed, types.WithSessionID(sessionID))
				if saveErr := s.chatRepo.SaveMessage(txCtx, systemMsg); saveErr != nil {
					return serrors.E(op, saveErr)
				}
				updated := currentSession.SetPreviousResponseID(nil, time.Now())
				if updateErr := s.chatRepo.UpdateSession(txCtx, updated); updateErr != nil {
					return serrors.E(op, updateErr)
				}
				return nil
			})
			if err != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, err), 0))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}

			if completeErr := s.completeRunState(persistCtx, session.TenantID(), sessionID, runID); completeErr != nil {
				active.Broadcast(streamingsvc.TerminalChunk(serrors.E(op, completeErr), time.Since(startedAt).Milliseconds()))
				_ = s.cancelRunState(persistCtx, session.TenantID(), sessionID, runID)
				return
			}
			active.Broadcast(streamingsvc.TerminalChunk(nil, time.Since(startedAt).Milliseconds()))
		},
	)
}
