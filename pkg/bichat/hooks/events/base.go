package events

import (
	"time"

	"github.com/google/uuid"
)

// baseEvent provides common fields for all event types.
// Embed this in concrete event structs.
type baseEvent struct {
	eventType string
	timestamp time.Time
	sessionID uuid.UUID
	tenantID  uuid.UUID
	traceID   string
}

// Type returns the event type identifier.
func (e *baseEvent) Type() string {
	return e.eventType
}

// Timestamp returns when the event occurred.
func (e *baseEvent) Timestamp() time.Time {
	return e.timestamp
}

// SessionID returns the chat session this event belongs to.
func (e *baseEvent) SessionID() uuid.UUID {
	return e.sessionID
}

// TenantID returns the tenant this event belongs to.
func (e *baseEvent) TenantID() uuid.UUID {
	return e.tenantID
}

// TraceID returns the logical trace/run identifier for this event.
// Empty means the event is session-scoped or predates trace-aware emission.
func (e *baseEvent) TraceID() string {
	return e.traceID
}

// newBaseEvent creates a baseEvent with current timestamp.
func newBaseEvent(eventType string, sessionID, tenantID uuid.UUID) baseEvent {
	return newBaseEventWithTrace(eventType, sessionID, tenantID, "")
}

// newBaseEventWithTrace creates a baseEvent with an explicit trace ID.
func newBaseEventWithTrace(eventType string, sessionID, tenantID uuid.UUID, traceID string) baseEvent {
	return baseEvent{
		eventType: eventType,
		timestamp: time.Now(),
		sessionID: sessionID,
		tenantID:  tenantID,
		traceID:   traceID,
	}
}
