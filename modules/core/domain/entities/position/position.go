package position

import (
	"time"
)

type Position struct {
	ID          int64
	Name        string
	Description *string
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}
