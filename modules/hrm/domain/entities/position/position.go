// Package position provides this package.
package position

import (
	"time"
)

type Position struct {
	ID          uint
	TenantID    string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
