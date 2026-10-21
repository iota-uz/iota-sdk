package notifications

import (
	"context"

	"github.com/google/uuid"
)

type GroupOption struct {
	ID   uuid.UUID
	Name string
}
type RoleOption struct {
	ID   uint
	Name string
}
type AudienceRepository interface {
	Groups(context.Context) ([]GroupOption, error)
	Roles(context.Context) ([]RoleOption, error)
	Resolve(context.Context, []uuid.UUID, []uint) ([]uint, error)
}
