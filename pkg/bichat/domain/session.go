// Package domain provides this package.
package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SessionStatus represents the status of a chat session.
type SessionStatus string

const (
	SessionStatusActive   SessionStatus = "ACTIVE"
	SessionStatusArchived SessionStatus = "ARCHIVED"
)

const SessionTitleMaxLen = 255

var (
	ErrInvalidSession            = errors.New("invalid session")
	ErrInvalidSessionTransition  = errors.New("invalid session transition")
	ErrInvalidSessionTitle       = errors.New("invalid session title")
	ErrInvalidSessionOwner       = errors.New("invalid session owner")
	ErrInvalidSessionTenant      = errors.New("invalid session tenant")
	ErrInvalidSessionStatus      = errors.New("invalid session status")
	ErrPinArchivedSession        = errors.New("cannot pin archived session")
	ErrArchiveArchivedSession    = errors.New("session is already archived")
	ErrUnarchiveActiveSession    = errors.New("session is already active")
	ErrCreateSessionWithArchived = errors.New("new session must start as active")
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

// SessionSpec validates constructor input for a new active session.
type SessionSpec struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	OwnerUserID           int64
	Title                 string
	ParentSessionID       *uuid.UUID
	LLMPreviousResponseID *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// SessionState hydrates a session from persistence.
type SessionState struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	OwnerUserID           int64
	Title                 string
	Status                SessionStatus
	Pinned                bool
	ParentSessionID       *uuid.UUID
	LLMPreviousResponseID *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Session represents a chat conversation aggregate root.
type Session interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	UserID() int64
	Title() string
	Status() SessionStatus
	Pinned() bool
	ParentSessionID() *uuid.UUID
	LLMPreviousResponseID() *string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	IsActive() bool
	IsArchived() bool
	IsPinned() bool
	HasParent() bool

	Archive(now time.Time) (Session, error)
	Unarchive(now time.Time) (Session, error)
	Rename(title string, now time.Time) (Session, error)
	Pin(now time.Time) (Session, error)
	Unpin(now time.Time) (Session, error)
	SetPreviousResponseID(responseID *string, now time.Time) Session
	Touch(now time.Time) Session
}

type session struct {
	id                    uuid.UUID
	tenantID              uuid.UUID
	userID                int64
	title                 string
	status                SessionStatus
	pinned                bool
	parentSessionID       *uuid.UUID
	llmPreviousResponseID *string
	createdAt             time.Time
	updatedAt             time.Time
}

func normalizeSessionTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		return "", ErrInvalidSessionTitle
	}
	if len([]rune(t)) > SessionTitleMaxLen {
		return "", ErrInvalidSessionTitle
	}
	return t, nil
}

// NewSession creates a new ACTIVE session with strict invariants.
func NewSession(spec SessionSpec) (Session, error) {
	if spec.TenantID == uuid.Nil {
		return nil, ErrInvalidSessionTenant
	}
	if spec.OwnerUserID <= 0 {
		return nil, ErrInvalidSessionOwner
	}
	title, err := normalizeSessionTitle(spec.Title)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	id := spec.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	createdAt := spec.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	updatedAt := spec.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	return &session{
		id:                    id,
		tenantID:              spec.TenantID,
		userID:                spec.OwnerUserID,
		title:                 title,
		status:                SessionStatusActive,
		pinned:                false,
		parentSessionID:       spec.ParentSessionID,
		llmPreviousResponseID: spec.LLMPreviousResponseID,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}, nil
}

// NewUntitledSession creates a session with an explicit default title.
func NewUntitledSession(spec SessionSpec) (Session, error) {
	spec.Title = "Untitled Session"
	return NewSession(spec)
}

// RehydrateSession creates a session aggregate from persisted state.
func RehydrateSession(state SessionState) (Session, error) {
	if state.ID == uuid.Nil {
		return nil, ErrInvalidSession
	}
	if state.TenantID == uuid.Nil {
		return nil, ErrInvalidSessionTenant
	}
	if state.OwnerUserID <= 0 {
		return nil, ErrInvalidSessionOwner
	}
	if !state.Status.Valid() {
		return nil, ErrInvalidSessionStatus
	}
	title := strings.TrimSpace(state.Title)
	if title == "" {
		title = "Untitled Session"
	}
	if len([]rune(title)) > SessionTitleMaxLen {
		return nil, ErrInvalidSessionTitle
	}
	createdAt := state.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := state.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	return &session{
		id:                    state.ID,
		tenantID:              state.TenantID,
		userID:                state.OwnerUserID,
		title:                 title,
		status:                state.Status,
		pinned:                state.Pinned,
		parentSessionID:       state.ParentSessionID,
		llmPreviousResponseID: state.LLMPreviousResponseID,
		createdAt:             createdAt,
		updatedAt:             updatedAt,
	}, nil
}

func (s *session) copy() *session {
	c := *s
	return &c
}

func sessionNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

func (s *session) ID() uuid.UUID                  { return s.id }
func (s *session) TenantID() uuid.UUID            { return s.tenantID }
func (s *session) UserID() int64                  { return s.userID }
func (s *session) Title() string                  { return s.title }
func (s *session) Status() SessionStatus          { return s.status }
func (s *session) Pinned() bool                   { return s.pinned }
func (s *session) ParentSessionID() *uuid.UUID    { return s.parentSessionID }
func (s *session) LLMPreviousResponseID() *string { return s.llmPreviousResponseID }
func (s *session) CreatedAt() time.Time           { return s.createdAt }
func (s *session) UpdatedAt() time.Time           { return s.updatedAt }

func (s *session) IsActive() bool   { return s.status.IsActive() }
func (s *session) IsArchived() bool { return s.status.IsArchived() }
func (s *session) IsPinned() bool   { return s.pinned }
func (s *session) HasParent() bool  { return s.parentSessionID != nil }

func (s *session) Archive(now time.Time) (Session, error) {
	if s.status != SessionStatusActive {
		return nil, ErrArchiveArchivedSession
	}
	c := s.copy()
	c.status = SessionStatusArchived
	c.updatedAt = sessionNow(now)
	return c, nil
}

func (s *session) Unarchive(now time.Time) (Session, error) {
	if s.status != SessionStatusArchived {
		return nil, ErrUnarchiveActiveSession
	}
	c := s.copy()
	c.status = SessionStatusActive
	c.updatedAt = sessionNow(now)
	return c, nil
}

func (s *session) Rename(title string, now time.Time) (Session, error) {
	normalized, err := normalizeSessionTitle(title)
	if err != nil {
		return nil, err
	}
	c := s.copy()
	c.title = normalized
	c.updatedAt = sessionNow(now)
	return c, nil
}

func (s *session) Pin(now time.Time) (Session, error) {
	if s.status != SessionStatusActive {
		return nil, ErrPinArchivedSession
	}
	c := s.copy()
	c.pinned = true
	c.updatedAt = sessionNow(now)
	return c, nil
}

func (s *session) Unpin(now time.Time) (Session, error) {
	c := s.copy()
	c.pinned = false
	c.updatedAt = sessionNow(now)
	return c, nil
}

func (s *session) SetPreviousResponseID(responseID *string, now time.Time) Session {
	c := s.copy()
	c.llmPreviousResponseID = responseID
	c.updatedAt = sessionNow(now)
	return c
}

func (s *session) Touch(now time.Time) Session {
	c := s.copy()
	c.updatedAt = sessionNow(now)
	return c
}
