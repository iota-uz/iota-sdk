package authlog

import (
	"time"

	"github.com/google/uuid"
)

type AuthenticationLog struct {
	ID        uint
	TenantID  uuid.UUID
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}
