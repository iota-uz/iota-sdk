package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the status of a chat session
type SessionStatus string

const (
	// SessionStatusActive indicates an active, usable session
	SessionStatusActive SessionStatus = "active"
	// SessionStatusArchived indicates an archived session
	SessionStatusArchived SessionStatus = "archived"
)

// String returns the string representation of SessionStatus
func (s SessionStatus) String() string {
	return string(s)
}

// IsActive returns true if the session is active
func (s SessionStatus) IsActive() bool {
	return s == SessionStatusActive
}

// IsArchived returns true if the session is archived
func (s SessionStatus) IsArchived() bool {
	return s == SessionStatusArchived
}

// Valid returns true if the status is a valid value
func (s SessionStatus) Valid() bool {
	switch s {
	case SessionStatusActive, SessionStatusArchived:
		return true
	default:
		return false
	}
}

// Session represents a chat conversation aggregate root.
// This is a struct (not interface) following idiomatic Go patterns.
type Session struct {
	ID                   uuid.UUID
	TenantID             uuid.UUID
	UserID               int64
	Title                string
	Status               SessionStatus
	Pinned               bool
	ParentSessionID      *uuid.UUID
	PendingQuestionAgent *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// NewSession creates a new session with the given parameters.
// Use functional options for optional fields.
func NewSession(opts ...SessionOption) *Session {
	s := &Session{
		ID:        uuid.New(),
		Status:    SessionStatusActive,
		Pinned:    false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// SessionOption is a functional option for creating sessions
type SessionOption func(*Session)

// WithID sets the session ID
func WithID(id uuid.UUID) SessionOption {
	return func(s *Session) {
		s.ID = id
	}
}

// WithTenantID sets the tenant ID
func WithTenantID(tenantID uuid.UUID) SessionOption {
	return func(s *Session) {
		s.TenantID = tenantID
	}
}

// WithUserID sets the user ID
func WithUserID(userID int64) SessionOption {
	return func(s *Session) {
		s.UserID = userID
	}
}

// WithTitle sets the session title
func WithTitle(title string) SessionOption {
	return func(s *Session) {
		s.Title = title
	}
}

// WithStatus sets the session status
func WithStatus(status SessionStatus) SessionOption {
	return func(s *Session) {
		s.Status = status
	}
}

// WithPinned sets the pinned status
func WithPinned(pinned bool) SessionOption {
	return func(s *Session) {
		s.Pinned = pinned
	}
}

// WithParentSessionID sets the parent session ID
func WithParentSessionID(parentID uuid.UUID) SessionOption {
	return func(s *Session) {
		s.ParentSessionID = &parentID
	}
}

// WithPendingQuestionAgent sets the pending question agent
func WithPendingQuestionAgent(agent string) SessionOption {
	return func(s *Session) {
		s.PendingQuestionAgent = &agent
	}
}

// IsActive returns true if the session is active
func (s *Session) IsActive() bool {
	return s.Status.IsActive()
}

// IsArchived returns true if the session is archived
func (s *Session) IsArchived() bool {
	return s.Status.IsArchived()
}

// IsPinned returns true if the session is pinned
func (s *Session) IsPinned() bool {
	return s.Pinned
}

// HasParent returns true if the session has a parent
func (s *Session) HasParent() bool {
	return s.ParentSessionID != nil
}

// HasPendingQuestion returns true if the session has a pending question
func (s *Session) HasPendingQuestion() bool {
	return s.PendingQuestionAgent != nil
}
