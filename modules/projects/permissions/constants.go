package permissions

import (
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceProject      permission.Resource = "project"
	ResourceProjectStage permission.Resource = "project_stage"
)

var (
	ProjectCreate = permission.MustCreate(
		uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7a"),
		"Project.Create",
		ResourceProject,
		permission.ActionCreate,
		"",
	)
	ProjectRead = permission.MustCreate(
		uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7b"),
		"Project.Read",
		ResourceProject,
		permission.ActionRead,
		"",
	)
	ProjectUpdate = permission.MustCreate(
		uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7c"),
		"Project.Update",
		ResourceProject,
		permission.ActionUpdate,
		"",
	)
	ProjectDelete = permission.MustCreate(
		uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7d"),
		"Project.Delete",
		ResourceProject,
		permission.ActionDelete,
		"",
	)

	ProjectStageCreate = permission.MustCreate(
		uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7a"),
		"ProjectStage.Create",
		ResourceProjectStage,
		permission.ActionCreate,
		"",
	)
	ProjectStageRead = permission.MustCreate(
		uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7b"),
		"ProjectStage.Read",
		ResourceProjectStage,
		permission.ActionRead,
		"",
	)
	ProjectStageUpdate = permission.MustCreate(
		uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7c"),
		"ProjectStage.Update",
		ResourceProjectStage,
		permission.ActionUpdate,
		"",
	)
	ProjectStageDelete = permission.MustCreate(
		uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7d"),
		"ProjectStage.Delete",
		ResourceProjectStage,
		permission.ActionDelete,
		"",
	)
)

var Permissions = []permission.Permission{
	ProjectCreate,
	ProjectRead,
	ProjectUpdate,
	ProjectDelete,
	ProjectStageCreate,
	ProjectStageRead,
	ProjectStageUpdate,
	ProjectStageDelete,
}
