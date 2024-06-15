package session

import (
	model "github.com/iota-agency/iota-erp/graph/gqlmodels"
	"time"
)

type Session struct {
	Token     string `gorm:"primaryKey"`
	UserId    int64
	Ip        string
	UserAgent string
	ExpiresAt time.Time
	CreatedAt time.Time
}

func (s *Session) IsExpired() bool {
	return s.ExpiresAt.Before(time.Now())
}

func (s *Session) ToGraph() *model.Session {
	return &model.Session{
		Token:     s.Token,
		UserID:    s.UserId,
		IP:        s.Ip,
		UserAgent: s.UserAgent,
		ExpiresAt: s.ExpiresAt,
		CreatedAt: s.CreatedAt,
	}
}
