package services

import (
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
)

const (
	EmployeeResource = "hrm.employee"
	PositionResource = "hrm.position"

	EmployeeCreated = "hrm.employee.created"
	EmployeeUpdated = "hrm.employee.updated"
	EmployeeDeleted = "hrm.employee.deleted"
	PositionCreated = "hrm.position.created"
	PositionUpdated = "hrm.position.updated"
	PositionDeleted = "hrm.position.deleted"
)

// HistoryDefinitions are the business history event types of the hrm module.
func HistoryDefinitions() []history.Definition {
	employeeRead := permissions.EmployeeRead
	defs := history.Lifecycle("hrm.employee", "Employees.History", history.Resource{Name: EmployeeResource, Fields: []history.Field{
		{Name: "first_name", LabelKey: "Employees.Public.FirstName.Label", Type: history.ValueString},
		{Name: "last_name", LabelKey: "Employees.Public.LastName.Label", Type: history.ValueString},
		{Name: "middle_name", LabelKey: "Employees.Public.MiddleName.Label", Type: history.ValueString},
		{Name: "email", LabelKey: "Employees.Public.Email.Label", Type: history.ValueString},
		{Name: "phone", LabelKey: "Employees.Public.Phone.Label", Type: history.ValueString},
		{Name: "hire_date", LabelKey: "Employees.Public.HireDate.Label", Type: history.ValueDate},
		{Name: "resignation_date", LabelKey: "Employees.Public.ResignationDate.Label", Type: history.ValueDate},
		{Name: "birth_date", LabelKey: "Employees.Public.BirthDate.Label", Type: history.ValueDate, ReadPermission: &employeeRead},
		{Name: "salary", LabelKey: "Employees.Private.Salary.Label", Type: history.ValueMoney, ReadPermission: &employeeRead},
		{Name: "tin", LabelKey: "Employees.Private.TIN.Label", Type: history.ValueString, ReadPermission: &employeeRead},
		{Name: "pin", LabelKey: "Employees.Private.Pin.Label", Type: history.ValueString, ReadPermission: &employeeRead},
	}})
	defs = append(defs, history.Lifecycle("hrm.position", "Positions.History", history.Resource{Name: PositionResource, Fields: []history.Field{
		{Name: "name", LabelKey: "Positions.Single.Name.Label", Type: history.ValueString},
		{Name: "description", LabelKey: "Positions.Single.Description.Label", Type: history.ValueString},
	}})...)
	return defs
}

func employeeSnapshot(e employee.Employee) history.Snapshot {
	s := history.Snapshot{
		"first_name":       e.FirstName(),
		"last_name":        e.LastName(),
		"middle_name":      e.MiddleName(),
		"phone":            e.Phone(),
		"hire_date":        history.Date(e.HireDate()),
		"birth_date":       history.Date(e.BirthDate()),
		"salary":           history.Money(e.Salary()),
		"email":            nil,
		"resignation_date": nil,
		"tin":              nil,
		"pin":              nil,
	}
	if e.Email() != nil {
		s["email"] = e.Email().Value()
	}
	if e.ResignationDate() != nil {
		s["resignation_date"] = history.Date(*e.ResignationDate())
	}
	if e.Tin() != nil {
		s["tin"] = e.Tin().Value()
	}
	if e.Pin() != nil {
		s["pin"] = e.Pin().Value()
	}
	return s
}

func positionSnapshot(p *position.Position) history.Snapshot {
	return history.Snapshot{
		"name":        p.Name,
		"description": p.Description,
	}
}
