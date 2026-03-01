// Package models provides this package.
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

var (
	ErrNilSessionModel = errors.New("session model is nil")
	ErrNilSession      = errors.New("session is nil")
)

// SessionModel is the database model for bichat.sessions.
type SessionModel struct {
	ID                    uuid.UUID
	TenantID              uuid.UUID
	UserID                int64
	Title                 string
	Status                string
	Pinned                bool
	ParentSessionID       *uuid.UUID
	LLMPreviousResponseID *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// ToDomain converts the model to a domain Session aggregate.
func (m *SessionModel) ToDomain() (domain.Session, error) {
	if m == nil {
		return nil, ErrNilSessionModel
	}
	return domain.RehydrateSession(domain.SessionState{
		ID:                    m.ID,
		TenantID:              m.TenantID,
		OwnerUserID:           m.UserID,
		Title:                 m.Title,
		Status:                domain.SessionStatus(m.Status),
		Pinned:                m.Pinned,
		ParentSessionID:       m.ParentSessionID,
		LLMPreviousResponseID: m.LLMPreviousResponseID,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
	})
}

// SessionModelFromDomain converts a domain Session aggregate to DB model.
func SessionModelFromDomain(s domain.Session) (*SessionModel, error) {
	if s == nil {
		return nil, ErrNilSession
	}

	var parentSessionID *uuid.UUID
	if s.ParentSessionID() != nil {
		parent := *s.ParentSessionID()
		parentSessionID = &parent
	}

	var llmPreviousResponseID *string
	if s.LLMPreviousResponseID() != nil {
		responseID := *s.LLMPreviousResponseID()
		llmPreviousResponseID = &responseID
	}

	return &SessionModel{
		ID:                    s.ID(),
		TenantID:              s.TenantID(),
		UserID:                s.UserID(),
		Title:                 s.Title(),
		Status:                s.Status().String(),
		Pinned:                s.Pinned(),
		ParentSessionID:       parentSessionID,
		LLMPreviousResponseID: llmPreviousResponseID,
		CreatedAt:             s.CreatedAt(),
		UpdatedAt:             s.UpdatedAt(),
	}, nil
}
