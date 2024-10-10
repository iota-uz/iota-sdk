package session

import (
	"github.com/iota-agency/iota-erp/internal/configuration"
	"time"

	"github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Session struct {
	Token     string `gorm:"primaryKey"`
	UserID    int64
	IP        string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateDTO struct {
	Token     string
	UserID    int64
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

func (s *Session) ToGraph() *model.Session {
	return &model.Session{
		Token:     s.Token,
		UserID:    s.UserID,
		IP:        s.IP,
		UserAgent: s.UserAgent,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}
