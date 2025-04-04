package role

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

type Role interface {
	ID() uint
	TenantID() uuid.UUID
	Name() string
	Description() string
	Permissions() []*permission.Permission
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetName(name string) Role
	SetDescription(description string) Role
	SetTenantID(tenantID uuid.UUID) Role

	AddPermission(p *permission.Permission) Role
	SetPermissions(permissions []*permission.Permission) Role
	Can(perm *permission.Permission) bool
}
