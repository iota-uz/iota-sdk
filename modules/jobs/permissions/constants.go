// Package permissions provides this package.
package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceJob permission.Resource = "job"
)

var (
	JobRun = permission.MustCreate(
		uuid.MustParse("c3d4e5f6-8a9b-4c0d-b1e2-6f7081929001"),
		"Job.Run",
		ResourceJob,
		permission.ActionCreate,
		"",
	)
	JobRead = permission.MustCreate(
		uuid.MustParse("c3d4e5f6-8a9b-4c0d-b1e2-6f7081929002"),
		"Job.Read",
		ResourceJob,
		permission.ActionRead,
		"",
	)
	JobDelete = permission.MustCreate(
		uuid.MustParse("c3d4e5f6-8a9b-4c0d-b1e2-6f7081929003"),
		"Job.Delete",
		ResourceJob,
		permission.ActionDelete,
		"",
	)
)

var Permissions = []permission.Permission{
	JobRun,
	JobRead,
	JobDelete,
}
