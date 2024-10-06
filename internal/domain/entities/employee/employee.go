package employee

import (
	"time"

	"github.com/iota-agency/iota-erp/internal/domain/entities/position"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Meta struct {
	EmployeeId        int64
	PrimaryLanguage   *string
	SecondaryLanguage *string
	Tin               *string
	BirthDate         *time.Time
	JoinDate          *time.Time
	LeaveDate         *time.Time
	GeneralInfo       *string
	YtProfileId       string
	UpdatedAt         *time.Time
}

func (e *Meta) ToGraph() *model.EmployeeMeta {
	return &model.EmployeeMeta{
		EmployeeID:        e.EmployeeId,
		PrimaryLanguage:   e.PrimaryLanguage,
		SecondaryLanguage: e.SecondaryLanguage,
		Tin:               e.Tin,
		BirthDate:         e.BirthDate,
		JoinDate:          e.JoinDate,
		LeaveDate:         e.LeaveDate,
		GeneralInfo:       e.GeneralInfo,
		YtProfileID:       &e.YtProfileId,
		UpdatedAt:         *e.UpdatedAt,
	}
}

type Employee struct {
	Id          int64
	FirstName   string
	LastName    string
	MiddleName  *string
	Email       string
	Phone       *string
	Salary      float64
	HourlyRate  float64
	PositionId  int64
	Coefficient float64
	Meta        *Meta
	Position    *position.Position
	AvatarId    *int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e *Employee) ToGraph() *model.Employee {
	return &model.Employee{
		ID:          e.Id,
		FirstName:   e.FirstName,
		LastName:    e.LastName,
		MiddleName:  e.MiddleName,
		Email:       e.Email,
		Phone:       e.Phone,
		Salary:      e.Salary,
		HourlyRate:  e.HourlyRate,
		Position:    e.Position.ToGraph(),
		Coefficient: e.Coefficient,
		Meta:        e.Meta.ToGraph(),
		AvatarID:    e.AvatarId,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
