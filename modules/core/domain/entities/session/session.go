package session

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type Session struct {
	Token     string `gorm:"primaryKey"`
	UserID    uint
	TenantID  uuid.UUID
	IP        string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateDTO struct {
	Token     string
	UserID    uint
	TenantID  uuid.UUID
	IP        string
	UserAgent string
}

func (d *CreateDTO) ToEntity() *Session {
	return &Session{
		Token:     d.Token,
		UserID:    d.UserID,
		TenantID:  d.TenantID,
		IP:        d.IP,
		UserAgent: d.UserAgent,
		ExpiresAt: time.Now().Add(configuration.Use().SessionDuration),
		CreatedAt: time.Now(),
	}
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}
