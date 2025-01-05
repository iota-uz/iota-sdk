package role

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"time"
)

type Role interface {
	ID() uint
	Name() string
	Description() string
	Permissions() []permission.Permission
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetName(name string) Role
	SetDescription(description string) Role

	AddPermission(p permission.Permission) Role
	Can(perm permission.Permission) bool
}
