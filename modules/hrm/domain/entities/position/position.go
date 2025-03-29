package position

import (
	"time"
)

type Position struct {
	ID          uint
	TenantID    uint
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
