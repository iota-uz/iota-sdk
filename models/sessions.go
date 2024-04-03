package models

import "time"

type Session struct {
	Token     string    `db:"token"`
	UserId    int64     `db:"user_id"`
	Ip        string    `db:"ip"`
	UserAgent string    `db:"user_agent"`
	ExpiresAt time.Time `db:"expires_at"`
	CreatedAt time.Time `db:"created_at"`
}
