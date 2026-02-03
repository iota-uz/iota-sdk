package domain

import (
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the status of a chat session
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "ACTIVE"
	SessionStatusArchived SessionStatus = "ARCHIVED"
)

func (s SessionStatus) String() string { return string(s) }

func (s SessionStatus) IsActive() bool   { return s == SessionStatusActive }
func (s SessionStatus) IsArchived() bool { return s == SessionStatusArchived }

func (s SessionStatus) Valid() bool {
	switch s {
	case SessionStatusActive, SessionStatusArchived:
		return true
	default:
		return false
	}
}

// Session represents a chat conversation aggregate root.
// Interface following the same design as other aggregates (e.g. ExpenseCategory, AuthLog).
type Session interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	UserID() int64
	Title() string
	Status() SessionStatus
	Pinned() bool
	ParentSessionID() *uuid.UUID
	PendingQuestionAgent() *string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	IsActive() bool
	IsArchived() bool
	IsPinned() bool
	HasParent() bool
	HasPendingQuestion() bool

	UpdateStatus(status SessionStatus) Session
	UpdateTitle(title string) Session
	UpdatePinned(pinned bool) Session
	UpdatePendingQuestionAgent(agent *string) Session
	UpdateUpdatedAt(t time.Time) Session
}

type session struct {
	id                   uuid.UUID
	tenantID             uuid.UUID
	userID               int64
	title                string
	status                SessionStatus
	pinned                bool
	parentSessionID      *uuid.UUID
	pendingQuestionAgent *string
	createdAt            time.Time
	updatedAt            time.Time
}

// SessionOption configures a session in NewSession.
type SessionOption func(*session)

func WithID(id uuid.UUID) SessionOption {
	return func(s *session) { s.id = id }
}
func WithTenantID(tenantID uuid.UUID) SessionOption {
	return func(s *session) { s.tenantID = tenantID }
}
func WithUserID(userID int64) SessionOption {
	return func(s *session) { s.userID = userID }
}
func WithTitle(title string) SessionOption {
	return func(s *session) { s.title = title }
}
func WithStatus(status SessionStatus) SessionOption {
	return func(s *session) { s.status = status }
}
func WithPinned(pinned bool) SessionOption {
	return func(s *session) { s.pinned = pinned }
}
func WithParentSessionID(parentID uuid.UUID) SessionOption {
	return func(s *session) { s.parentSessionID = &parentID }
}
func WithPendingQuestionAgent(agent string) SessionOption {
	return func(s *session) { s.pendingQuestionAgent = &agent }
}
func WithCreatedAt(t time.Time) SessionOption {
	return func(s *session) { s.createdAt = t }
}
func WithUpdatedAt(t time.Time) SessionOption {
	return func(s *session) { s.updatedAt = t }
}

// NewSession creates a new session with the given options.
func NewSession(opts ...SessionOption) Session {
	s := &session{
		id:        uuid.New(),
		status:    SessionStatusActive,
		pinned:    false,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *session) ID() uuid.UUID                      { return s.id }
func (s *session) TenantID() uuid.UUID                { return s.tenantID }
func (s *session) UserID() int64                      { return s.userID }
func (s *session) Title() string                      { return s.title }
func (s *session) Status() SessionStatus               { return s.status }
func (s *session) Pinned() bool                       { return s.pinned }
func (s *session) ParentSessionID() *uuid.UUID        { return s.parentSessionID }
func (s *session) PendingQuestionAgent() *string     { return s.pendingQuestionAgent }
func (s *session) CreatedAt() time.Time               { return s.createdAt }
func (s *session) UpdatedAt() time.Time               { return s.updatedAt }

func (s *session) IsActive() bool   { return s.status.IsActive() }
func (s *session) IsArchived() bool { return s.status.IsArchived() }
func (s *session) IsPinned() bool   { return s.pinned }
func (s *session) HasParent() bool  { return s.parentSessionID != nil }
func (s *session) HasPendingQuestion() bool { return s.pendingQuestionAgent != nil }

func (s *session) UpdateStatus(status SessionStatus) Session {
	c := *s
	c.status = status
	c.updatedAt = time.Now()
	return &c
}

func (s *session) UpdateTitle(title string) Session {
	c := *s
	c.title = title
	c.updatedAt = time.Now()
	return &c
}

func (s *session) UpdatePinned(pinned bool) Session {
	c := *s
	c.pinned = pinned
	c.updatedAt = time.Now()
	return &c
}

func (s *session) UpdatePendingQuestionAgent(agent *string) Session {
	c := *s
	c.pendingQuestionAgent = agent
	c.updatedAt = time.Now()
	return &c
}

func (s *session) UpdateUpdatedAt(t time.Time) Session {
	c := *s
	c.updatedAt = t
	return &c
}
