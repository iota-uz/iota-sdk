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
	ProjectCreate = &permission.Permission{
		ID:       uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7a"),
		Name:     "Project.Create",
		Resource: ResourceProject,
		Action:   permission.ActionCreate,
	}
	ProjectRead = &permission.Permission{
		ID:       uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7b"),
		Name:     "Project.Read",
		Resource: ResourceProject,
		Action:   permission.ActionRead,
	}
	ProjectUpdate = &permission.Permission{
		ID:       uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7c"),
		Name:     "Project.Update",
		Resource: ResourceProject,
		Action:   permission.ActionUpdate,
	}
	ProjectDelete = &permission.Permission{
		ID:       uuid.MustParse("a1b2c3d4-1bb5-4e26-9817-a7787011eb7d"),
		Name:     "Project.Delete",
		Resource: ResourceProject,
		Action:   permission.ActionDelete,
	}

	ProjectStageCreate = &permission.Permission{
		ID:       uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7a"),
		Name:     "ProjectStage.Create",
		Resource: ResourceProjectStage,
		Action:   permission.ActionCreate,
	}
	ProjectStageRead = &permission.Permission{
		ID:       uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7b"),
		Name:     "ProjectStage.Read",
		Resource: ResourceProjectStage,
		Action:   permission.ActionRead,
	}
	ProjectStageUpdate = &permission.Permission{
		ID:       uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7c"),
		Name:     "ProjectStage.Update",
		Resource: ResourceProjectStage,
		Action:   permission.ActionUpdate,
	}
	ProjectStageDelete = &permission.Permission{
		ID:       uuid.MustParse("b1c2d3e4-1bb5-4e26-9817-a7787011eb7d"),
		Name:     "ProjectStage.Delete",
		Resource: ResourceProjectStage,
		Action:   permission.ActionDelete,
	}
)

var Permissions = []*permission.Permission{
	ProjectCreate,
	ProjectRead,
	ProjectUpdate,
	ProjectDelete,
	ProjectStageCreate,
	ProjectStageRead,
	ProjectStageUpdate,
	ProjectStageDelete,
}
