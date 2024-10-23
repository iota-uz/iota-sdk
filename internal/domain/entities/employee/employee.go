package employee

import (
	"github.com/iota-agency/iota-erp/sdk/mapper"
	"time"

	"github.com/iota-agency/iota-erp/internal/domain/entities/position"
	model "github.com/iota-agency/iota-erp/internal/interfaces/graph/gqlmodels"
)

type Meta struct {
	EmployeeID        uint
	PrimaryLanguage   *string
	SecondaryLanguage *string
	Tin               *string
	BirthDate         *time.Time
	JoinDate          *time.Time
	LeaveDate         *time.Time
	GeneralInfo       *string
	YtProfileID       string
	UpdatedAt         *time.Time
}

func (e *Meta) ToGraph() *model.EmployeeMeta {
	return &model.EmployeeMeta{
		EmployeeID:        int64(e.EmployeeID),
		PrimaryLanguage:   e.PrimaryLanguage,
		SecondaryLanguage: e.SecondaryLanguage,
		Tin:               e.Tin,
		BirthDate:         e.BirthDate,
		JoinDate:          e.JoinDate,
		LeaveDate:         e.LeaveDate,
		GeneralInfo:       e.GeneralInfo,
		YtProfileID:       &e.YtProfileID,
		UpdatedAt:         *e.UpdatedAt,
	}
}

type Employee struct {
	ID          uint
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	HourlyRate  float64
	PositionID  uint
	Coefficient float64
	Meta        *Meta
	Position    *position.Position
	AvatarID    uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (e *Employee) ToGraph() *model.Employee {
	return &model.Employee{
		ID:          int64(e.ID),
		FirstName:   e.FirstName,
		LastName:    e.LastName,
		MiddleName:  &e.MiddleName,
		Email:       e.Email,
		Phone:       &e.Phone,
		Salary:      e.Salary,
		HourlyRate:  e.HourlyRate,
		Position:    e.Position.ToGraph(),
		Coefficient: e.Coefficient,
		Meta:        e.Meta.ToGraph(),
		AvatarID:    mapper.Pointer(int64(e.AvatarID)),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}
