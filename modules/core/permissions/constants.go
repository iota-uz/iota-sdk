package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceUser     permission.Resource = "user"
	ResourceRole     permission.Resource = "role"
	ResourceEmployee permission.Resource = "employee"
	ResourceUpload   permission.Resource = "upload"
)

var (
	UserCreate = &permission.Permission{
		ID:       uuid.MustParse("8b6060b3-af5e-4ae0-b32d-b33695141066"),
		Name:     "User.Create",
		Resource: ResourceUser,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	UserRead = &permission.Permission{
		ID:       uuid.MustParse("13f011c8-1107-4957-ad19-70cfc167a775"),
		Name:     "User.Read",
		Resource: ResourceUser,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	UserUpdate = &permission.Permission{
		ID:       uuid.MustParse("1c351fd3-9a2b-40b9-80b1-11ba81e645c8"),
		Name:     "User.Update",
		Resource: ResourceUser,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	UserDelete = &permission.Permission{
		ID:       uuid.MustParse("547cded3-6754-4a05-aeb0-a38d12ed05ee"),
		Name:     "User.Delete",
		Resource: ResourceUser,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
	RoleCreate = &permission.Permission{
		ID:       uuid.MustParse("60f195ed-d373-41c3-a39d-bb7484850840"),
		Name:     "Role.Create",
		Resource: ResourceRole,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	RoleRead = &permission.Permission{
		ID:       uuid.MustParse("51d1025e-11fe-405e-9ab4-88078c28e110"),
		Name:     "Role.Read",
		Resource: ResourceRole,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	RoleUpdate = &permission.Permission{
		ID:       uuid.MustParse("ea18e9d1-6ac4-4b2a-861c-cc89d95d7a19"),
		Name:     "Role.Update",
		Resource: ResourceRole,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	RoleDelete = &permission.Permission{
		ID:       uuid.MustParse("5fcea09b-913e-4bbf-bb00-66586c29e930"),
		Name:     "Role.Delete",
		Resource: ResourceRole,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
	EmployeeCreate = &permission.Permission{
		ID:       uuid.MustParse("8a19d587-8411-492b-80bd-cd037bd1b41b"),
		Name:     "Employee.Create",
		Resource: ResourceEmployee,
		Action:   permission.ActionCreate,
		Modifier: permission.ModifierAll,
	}
	EmployeeRead = &permission.Permission{
		ID:       uuid.MustParse("6592f625-4bdf-4a85-940c-1815f49ee5ba"),
		Name:     "Employee.Read",
		Resource: ResourceEmployee,
		Action:   permission.ActionRead,
		Modifier: permission.ModifierAll,
	}
	EmployeeUpdate = &permission.Permission{
		ID:       uuid.MustParse("e46d0080-8919-447b-bf90-a4930c2d0ab5"),
		Name:     "Employee.Update",
		Resource: ResourceEmployee,
		Action:   permission.ActionUpdate,
		Modifier: permission.ModifierAll,
	}
	EmployeeDelete = &permission.Permission{
		ID:       uuid.MustParse("dc632571-a97f-423d-8892-2ef2176be79b"),
		Name:     "Employee.Delete",
		Resource: ResourceEmployee,
		Action:   permission.ActionDelete,
		Modifier: permission.ModifierAll,
	}
)

var Permissions = []*permission.Permission{
	UserCreate,
	UserRead,
	UserUpdate,
	UserDelete,
	RoleCreate,
	RoleRead,
	RoleUpdate,
	RoleDelete,
	EmployeeCreate,
	EmployeeRead,
	EmployeeUpdate,
	EmployeeDelete,
}
