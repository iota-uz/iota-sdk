package mappers

import (
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/presentation/viewmodels"
)

func EmployeeToViewModel(entity employee.Employee) *viewmodels.Employee {
	var email, pin, tin string
	if entity.Email() != nil {
		email = entity.Email().Value()
	}
	if entity.Pin() != nil {
		pin = entity.Pin().Value()
	}
	if entity.Tin() != nil {
		tin = entity.Tin().Value()
	}

	return &viewmodels.Employee{
		ID:              strconv.FormatUint(uint64(entity.ID()), 10),
		FirstName:       entity.FirstName(),
		LastName:        entity.LastName(),
		MiddleName:      entity.MiddleName(),
		Email:           email,
		Salary:          strconv.FormatFloat(entity.Salary().Value(), 'f', 2, 64),
		Phone:           entity.Phone(),
		Pin:             pin,
		Tin:             tin,
		BirthDate:       entity.BirthDate().Format(time.DateOnly),
		HireDate:        entity.HireDate().Format(time.DateOnly),
		ResignationDate: entity.BirthDate().Format(time.DateOnly),
		Notes:           entity.Notes(),
		UpdatedAt:       entity.UpdatedAt().Format(time.RFC3339),
		CreatedAt:       entity.CreatedAt().Format(time.RFC3339),
	}
}
