package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceLogs permission.Resource = "logs"
)

var (
	ViewLogs = &permission.Permission{
		ID:       uuid.MustParse("7d9454d8-607e-4f30-bc12-459bbcc939b5"),
		Name:     "Logs.View",
		Resource: ResourceLogs,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
)

var Permissions = []*permission.Permission{
	ViewLogs,
}
