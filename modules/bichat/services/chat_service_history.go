package services

import (
	"context"
	"time"

	"github.com/google/uuid"
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
