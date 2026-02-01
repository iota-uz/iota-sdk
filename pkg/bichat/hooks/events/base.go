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

// newBaseEvent creates a baseEvent with current timestamp.
func newBaseEvent(eventType string, sessionID, tenantID uuid.UUID) baseEvent {
	return baseEvent{
		eventType: eventType,
		timestamp: time.Now(),
		sessionID: sessionID,
		tenantID:  tenantID,
	}
}
