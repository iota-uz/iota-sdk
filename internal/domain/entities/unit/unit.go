package unit

import (
	"time"
)

type Unit struct {
	ID          int
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
