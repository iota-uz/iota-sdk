package permissions

import (
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
)

const (
	ResourceEmployee permission.Resource = "employee"
)

var (
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
	EmployeeCreate,
	EmployeeRead,
	EmployeeUpdate,
	EmployeeDelete,
}
