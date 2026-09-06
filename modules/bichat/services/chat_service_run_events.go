package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/services/streaming"
	api "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
)

func (s *chatServiceImpl) appendRunEvent(ctx context.Context, tenantID, sessionID, runID uuid.UUID, chunk api.StreamChunk) error {
	const op serrors.Op = "chatServiceImpl.appendRunEvent"
	if s.eventLog == nil {
		return nil
	}
	eventType, body, err := encodeRunEventFromChunk(chunk)
	if err == nil {
		_, err = s.eventLog.Append(ctx, tenantID, runID, RunEvent{Type: eventType, Payload: body})
	}
	if err != nil {
		s.log().WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID.String(), "session_id": sessionID.String(), "run_id": runID.String(),
			"stage": "event_log_append", "event_type": string(chunk.Type),
		}).Error("bichat: failed to persist run event")
		return serrors.E(op, err)
	}
	return nil
}

func (s *chatServiceImpl) mirrorRunEvents(ctx context.Context, tenantID uuid.UUID, active *streaming.ActiveRun) {
	if s.eventLog == nil {
		return
	}
	active.SetMirror(func(chunk api.StreamChunk) {
		// Keep the in-memory stream alive on journal failure; appendRunEvent logs
		// the failure, including terminal events, with the run's correlation IDs.
		_ = s.appendRunEvent(ctx, tenantID, active.SessionID, active.RunID, chunk)
	})
}

func (s *chatServiceImpl) expireRunEvents(ctx context.Context, tenantID, sessionID, runID uuid.UUID) {
	if s.eventLog == nil {
		return
	}
	if err := s.eventLog.DropAfterTerminal(ctx, tenantID, runID, 5*time.Minute); err != nil {
		s.log().WithError(err).WithFields(logrus.Fields{
			"tenant_id": tenantID.String(), "session_id": sessionID.String(), "run_id": runID.String(),
			"stage": "event_log_expire",
		}).Warn("bichat: failed to expire run events")
	}
}
