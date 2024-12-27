package authlog

import (
	"time"
)

type AuthenticationLog struct {
	ID        uint
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}
