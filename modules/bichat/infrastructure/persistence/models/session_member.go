// Package models provides this package.
package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

var (
	ErrNilSessionMemberModel = errors.New("session member model is nil")
)

// SessionMemberModel is a row projection for bichat.session_members + user names.
type SessionMemberModel struct {
	SessionID uuid.UUID
	UserID    int64
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
	FirstName string
	LastName  string
}

// ToDomain converts the model to a domain SessionMember.
func (m *SessionMemberModel) ToDomain() (domain.SessionMember, error) {
	if m == nil {
		return domain.SessionMember{}, ErrNilSessionMemberModel
	}
	return domain.NewSessionMember(domain.SessionMemberSpec{
		SessionID: m.SessionID,
		User: domain.SessionUser{
			ID:        m.UserID,
			FirstName: m.FirstName,
			LastName:  m.LastName,
		},
		Role:      domain.ParseSessionMemberRole(m.Role),
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	})
}

// SessionMemberUpsertModel is the write model for upsert.
type SessionMemberUpsertModel struct {
	SessionID uuid.UUID
	UserID    int64
	Role      string
}

// SessionMemberUpsertModelFromDomain converts a validated upsert command to write model.
func SessionMemberUpsertModelFromDomain(command domain.SessionMemberUpsert) *SessionMemberUpsertModel {
	return &SessionMemberUpsertModel{
		SessionID: command.SessionID(),
		UserID:    command.UserID(),
		Role:      command.Role().String(),
	}
}

// SessionMemberRemovalModel is the write model for delete.
type SessionMemberRemovalModel struct {
	SessionID uuid.UUID
	UserID    int64
}

// SessionMemberRemovalModelFromDomain converts a validated removal command to write model.
func SessionMemberRemovalModelFromDomain(command domain.SessionMemberRemoval) *SessionMemberRemovalModel {
	return &SessionMemberRemovalModel{
		SessionID: command.SessionID(),
		UserID:    command.UserID(),
	}
}
