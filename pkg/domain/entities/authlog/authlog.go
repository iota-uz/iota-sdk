package authlog

import (
	"time"
)

type AuthenticationLog struct {
	ID        int64
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}
