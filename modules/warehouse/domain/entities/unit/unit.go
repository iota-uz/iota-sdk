package unit

import (
	"time"

	"github.com/google/uuid"
)

type Unit struct {
	ID         uint
	TenantID   uuid.UUID
	Title      string
	ShortTitle string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
