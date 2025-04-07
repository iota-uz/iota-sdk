package tab

import (
	"github.com/google/uuid"
)

type Tab struct {
	ID       uint
	Href     string
	UserID   uint
	Position uint
	TenantID uuid.UUID
}
