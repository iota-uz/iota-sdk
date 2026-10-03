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
	_, err := s.appendRunEventWithID(ctx, tenantID, sessionID, runID, chunk)
	return err
}

// appendRunEventWithID journals the chunk and returns the event-log stream
// id so callers can tail exclusively after it (avoids replaying the event
// they just delivered directly).
func (s *chatServiceImpl) appendRunEventWithID(ctx context.Context, tenantID, sessionID, runID uuid.UUID, chunk api.StreamChunk) (string, error) {
	const op serrors.Op = "chatServiceImpl.appendRunEventWithID"
	if s.eventLog == nil {
		return "", nil
	}
	eventType, body, err := encodeRunEventFromChunk(chunk)
	if err == nil {
		var streamID string
		streamID, err = s.eventLog.Append(ctx, tenantID, runID, RunEvent{Type: eventType, Payload: body})
		if err == nil {
			return streamID, nil
		}
	}
	s.log().WithError(err).WithFields(logrus.Fields{
		"tenant_id": tenantID.String(), "session_id": sessionID.String(), "run_id": runID.String(),
		"stage": "event_log_append", "event_type": string(chunk.Type),
	}).Error("bichat: failed to persist run event")
	return "", serrors.Wrap(op, err)
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
	const op serrors.Op = "chatServiceImpl.expireRunEvents"
	if s.eventLog == nil {
		return
	}
	if err := s.eventLog.DropAfterTerminal(ctx, tenantID, runID, 5*time.Minute); err != nil {
		s.log().WithError(serrors.Wrap(op, err)).WithFields(logrus.Fields{
			"tenant_id": tenantID.String(), "session_id": sessionID.String(), "run_id": runID.String(),
			"stage": "event_log_expire",
		}).Warn("bichat: failed to expire run events")
	}
}
