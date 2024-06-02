package models

import (
	model "github.com/iota-agency/iota-erp/graph/gqlmodels"
	"time"
)

type Session struct {
	Token     string    `db:"token"`
	UserId    int64     `db:"user_id"`
	Ip        string    `db:"ip"`
	UserAgent string    `db:"user_agent"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
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
