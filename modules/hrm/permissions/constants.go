package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceEmployee permission.Resource = "employee"
)

var (
	EmployeeCreate = permission.MustCreate(
		uuid.MustParse("8a19d587-8411-492b-80bd-cd037bd1b41b"),
		"Employee.Create",
		ResourceEmployee,
		permission.ActionCreate,
		permission.ModifierAll,
	)
	EmployeeRead = permission.MustCreate(
		uuid.MustParse("6592f625-4bdf-4a85-940c-1815f49ee5ba"),
		"Employee.Read",
		ResourceEmployee,
		permission.ActionRead,
		permission.ModifierAll,
	)
	EmployeeUpdate = permission.MustCreate(
		uuid.MustParse("e46d0080-8919-447b-bf90-a4930c2d0ab5"),
		"Employee.Update",
		ResourceEmployee,
		permission.ActionUpdate,
		permission.ModifierAll,
	)
	EmployeeDelete = permission.MustCreate(
		uuid.MustParse("dc632571-a97f-423d-8892-2ef2176be79b"),
		"Employee.Delete",
		ResourceEmployee,
		permission.ActionDelete,
		permission.ModifierAll,
	)
)

var Permissions = []permission.Permission{
	EmployeeCreate,
	EmployeeRead,
	EmployeeUpdate,
	EmployeeDelete,
}
