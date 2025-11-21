package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceLogs permission.Resource = "logs"
)

var (
	ViewLogs = permission.MustCreate(
		uuid.MustParse("6513b6fa-b8fb-42df-9cbd-f468b2220762"),
		"Logs.View",
		ResourceLogs,
		permission.ActionRead,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	ViewLogs,
}
