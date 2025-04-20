package session

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type Session struct {
	Token     string `gorm:"primaryKey"`
	UserID    uint
	IP        string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateDTO struct {
	Token     string
	UserID    uint
	IP        string
	UserAgent string
}

func (d *CreateDTO) ToEntity() *Session {
	return &Session{
		Token:     d.Token,
		UserID:    d.UserID,
		IP:        d.IP,
		UserAgent: d.UserAgent,
		ExpiresAt: time.Now().Add(configuration.Use().SessionDuration),
		CreatedAt: time.Now(),
	}
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}
