package models

import (
	"database/sql"
	"time"
)

type Position struct {
	ID          uint
	TenantID    string
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Employee struct {
	ID               uint
	TenantID         string
	FirstName        string
	LastName         string
	MiddleName       sql.NullString
	Email            string
	Phone            sql.NullString
	Salary           float64
	SalaryCurrencyID sql.NullString
	HourlyRate       float64
	Coefficient      float64
	AvatarID         *uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EmployeeMeta struct {
	PrimaryLanguage   sql.NullString
	SecondaryLanguage sql.NullString
	Tin               sql.NullString
	Pin               sql.NullString
	Notes             sql.NullString
	BirthDate         sql.NullTime
	HireDate          sql.NullTime
	ResignationDate   sql.NullTime
}

type EmployeePosition struct {
	EmployeeID uint
	PositionID uint
}
