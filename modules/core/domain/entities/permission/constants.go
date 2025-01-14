package permission

import "github.com/google/uuid"

const (
	ResourceUser     Resource = "user"
	ResourceRole     Resource = "role"
	ResourceAccount  Resource = "account"
	ResourceStage    Resource = "stage"
	ResourceProject  Resource = "project"
	ResourceEmployee Resource = "employee"
	ResourceSetting  Resource = "settings"
	ResourceUpload   Resource = "upload"
)

const (
	ActionCreate Action = "create"
	ActionRead   Action = "read"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

const (
	ModifierAll Modifier = "all"
	ModifierOwn Modifier = "own"
)

var (
	UserCreate = &Permission{
		ID:       uuid.MustParse("8b6060b3-af5e-4ae0-b32d-b33695141066"),
		Name:     "User.Create",
		Resource: ResourceUser,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	UserRead = &Permission{
		ID:       uuid.MustParse("13f011c8-1107-4957-ad19-70cfc167a775"),
		Name:     "User.Read",
		Resource: ResourceUser,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	UserUpdate = &Permission{
		ID:       uuid.MustParse("1c351fd3-9a2b-40b9-80b1-11ba81e645c8"),
		Name:     "User.Update",
		Resource: ResourceUser,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	UserDelete = &Permission{
		ID:       uuid.MustParse("547cded3-6754-4a05-aeb0-a38d12ed05ee"),
		Name:     "User.Delete",
		Resource: ResourceUser,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	RoleCreate = &Permission{
		ID:       uuid.MustParse("60f195ed-d373-41c3-a39d-bb7484850840"),
		Name:     "Role.Create",
		Resource: ResourceRole,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	RoleRead = &Permission{
		ID:       uuid.MustParse("51d1025e-11fe-405e-9ab4-88078c28e110"),
		Name:     "Role.Read",
		Resource: ResourceRole,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	RoleUpdate = &Permission{
		ID:       uuid.MustParse("ea18e9d1-6ac4-4b2a-861c-cc89d95d7a19"),
		Name:     "Role.Update",
		Resource: ResourceRole,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	RoleDelete = &Permission{
		ID:       uuid.MustParse("5fcea09b-913e-4bbf-bb00-66586c29e930"),
		Name:     "Role.Delete",
		Resource: ResourceRole,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	AccountCreate = &Permission{
		ID:       uuid.MustParse("f4952b94-d3a9-4f89-8449-827f9bde95c9"),
		Name:     "Account.Create",
		Resource: ResourceAccount,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	AccountRead = &Permission{
		ID:       uuid.MustParse("8e47b8eb-2707-458f-b4b5-607654ef26a0"),
		Name:     "Account.Read",
		Resource: ResourceAccount,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	AccountUpdate = &Permission{
		ID:       uuid.MustParse("274b7dd8-0d0c-4fc4-9bbb-1658e98c05ac"),
		Name:     "Account.Update",
		Resource: ResourceAccount,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	AccountDelete = &Permission{
		ID:       uuid.MustParse("715811f0-b6ce-4333-a45e-904dba06d693"),
		Name:     "Account.Delete",
		Resource: ResourceAccount,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	StageCreate = &Permission{
		ID:       uuid.MustParse("dcafc818-6c9e-410a-977b-70d649e7c1f7"),
		Name:     "Stage.Create",
		Resource: ResourceStage,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	StageRead = &Permission{
		ID:       uuid.MustParse("b9448c1c-14e3-4c59-913b-235b86fbf09e"),
		Name:     "Stage.Read",
		Resource: ResourceStage,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	StageUpdate = &Permission{
		ID:       uuid.MustParse("975760a4-9b77-48ee-8f02-15f168e55173"),
		Name:     "Stage.Update",
		Resource: ResourceStage,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	StageDelete = &Permission{
		ID:       uuid.MustParse("7c70aac3-9805-4b42-b0cd-4961e354b495"),
		Name:     "Stage.Delete",
		Resource: ResourceStage,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	ProjectCreate = &Permission{
		ID:       uuid.MustParse("e203c7a2-9f28-4fbc-b941-b184938c3ade"),
		Name:     "Project.Create",
		Resource: ResourceProject,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	ProjectRead = &Permission{
		ID:       uuid.MustParse("8eca75bf-320f-4e1d-b06b-28cb32370a2b"),
		Name:     "Project.Read",
		Resource: ResourceProject,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	ProjectUpdate = &Permission{
		ID:       uuid.MustParse("934d334c-4945-4676-a80d-ae6de1c52399"),
		Name:     "Project.Update",
		Resource: ResourceProject,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	ProjectDelete = &Permission{
		ID:       uuid.MustParse("32d65838-70fd-4d59-aa86-22177b1cedc1"),
		Name:     "Project.Delete",
		Resource: ResourceProject,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	EmployeeCreate = &Permission{
		ID:       uuid.MustParse("8a19d587-8411-492b-80bd-cd037bd1b41b"),
		Name:     "Employee.Create",
		Resource: ResourceEmployee,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	EmployeeRead = &Permission{
		ID:       uuid.MustParse("6592f625-4bdf-4a85-940c-1815f49ee5ba"),
		Name:     "Employee.Read",
		Resource: ResourceEmployee,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
	EmployeeUpdate = &Permission{
		ID:       uuid.MustParse("e46d0080-8919-447b-bf90-a4930c2d0ab5"),
		Name:     "Employee.Update",
		Resource: ResourceEmployee,
		Action:   ActionUpdate,
		Modifier: ModifierAll,
	}
	EmployeeDelete = &Permission{
		ID:       uuid.MustParse("dc632571-a97f-423d-8892-2ef2176be79b"),
		Name:     "Employee.Delete",
		Resource: ResourceEmployee,
		Action:   ActionDelete,
		Modifier: ModifierAll,
	}
	SettingsUpdate = &Permission{
		ID:       uuid.MustParse("842d5906-ad25-4cb6-b64a-ec14cf9acb25"),
		Name:     "Settings.Update",
		Resource: ResourceSetting,
		Action:   ActionCreate,
		Modifier: ModifierAll,
	}
	SettingsRead = &Permission{
		ID:       uuid.MustParse("0013399c-4974-4e26-be80-8f90d4357c24"),
		Name:     "Settings.Read",
		Resource: ResourceSetting,
		Action:   ActionRead,
		Modifier: ModifierAll,
	}
)

var Permissions = []*Permission{
	UserCreate,
	UserRead,
	UserUpdate,
	UserDelete,
	RoleCreate,
	RoleRead,
	RoleUpdate,
	RoleDelete,
	AccountCreate,
	AccountRead,
	AccountUpdate,
	AccountDelete,
	ProjectCreate,
	ProjectRead,
	ProjectUpdate,
	ProjectDelete,
	StageCreate,
	StageRead,
	StageUpdate,
	StageDelete,
	EmployeeCreate,
	EmployeeRead,
	EmployeeUpdate,
	EmployeeDelete,
	SettingsUpdate,
	SettingsRead,
}
