package unit

import (
	"time"
)

type Unit struct {
	ID         uint
	TenantID   uint
	Title      string
	ShortTitle string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
