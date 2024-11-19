package employee

import (
	"time"

	"github.com/iota-agency/iota-sdk/pkg/domain/entities/position"
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

type Employee struct {
	ID          uint
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	Salary      float64
	HourlyRate  float64
	Coefficient float64
	Meta        *Meta
	Position    *position.Position
	AvatarID    uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
