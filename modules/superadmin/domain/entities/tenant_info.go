package entities

import (
	"time"

	"github.com/google/uuid"
)

// TenantInfo represents a tenant with additional metadata for superadmin view
type TenantInfo struct {
	ID        uuid.UUID
	Name      string
	Domain    string
	UserCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}
