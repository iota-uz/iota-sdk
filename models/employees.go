package models

import "time"

type EmployeeMeta struct {
	EmployeeId        int64          `db:"employee_id" gql:"employee_id"`
	PrimaryLanguage   JsonNullString `db:"primary_language" gql:"primary_language"`
	SecondaryLanguage JsonNullString `db:"secondary_language" gql:"secondary_language"`
	Tin               JsonNullString `db:"tin" gql:"tin"`
	BirthDate         *time.Time     `db:"birth_date" gql:"birth_date"`
	JoinDate          *time.Time     `db:"join_date" gql:"join_date"`
	LeaveDate         *time.Time     `db:"leave_date" gql:"leave_date"`
	GeneralInfo       JsonNullString `db:"general_info" gql:"general_info"`
	YtProfileId       string         `db:"yt_profile_id" gql:"yt_profile_id"`
	UpdatedAt         *time.Time     `db:"updated_at" gql:"updated_at"`
}

func (e *EmployeeMeta) PkField() *Field {
	return &Field{
		Name: "employee_id",
		Type: BigInt,
	}
}

func (e *EmployeeMeta) Table() string {
	return "employee_meta"
}

func (e *EmployeeMeta) Pk() interface{} {
	return e.EmployeeId
}

type Position struct {
	Id          int64          `db:"id" gql:"id"`
	Name        string         `db:"name" gql:"name"`
	Description JsonNullString `db:"description" gql:"description"`
	CreatedAt   *time.Time     `db:"created_at" gql:"created_at"`
	UpdatedAt   *time.Time     `db:"updated_at" gql:"updated_at"`
}

func (p *Position) PkField() *Field {
	return &Field{
		Name: "id",
		Type: BigSerial,
	}
}

func (p *Position) Table() string {
	return "positions"
}

func (p *Position) Pk() interface{} {
	return p.Id
}

type Employee struct {
	Id          int64          `db:"id" gql:"id"`
	FirstName   string         `db:"first_name" gql:"first_name"`
	LastName    string         `db:"last_name" gql:"last_name"`
	MiddleName  JsonNullString `db:"middle_name" gql:"middle_name"`
	Email       string         `db:"email" gql:"email"`
	Phone       JsonNullString `db:"phone" gql:"phone"`
	Salary      float64        `db:"salary" gql:"salary"`
	HourlyRate  float64        `db:"hourly_rate" gql:"hourly_rate"`
	PositionId  int64          `db:"position_id" gql:"position_id"`
	Coefficient float64        `db:"coefficient" gql:"coefficient"`
	Meta        *EmployeeMeta  `db:"id" gql:"meta" belongs_to:"employee_id"`
	Position    *Position      `db:"position_id" gql:"position" belongs_to:"id"`
	AvatarId    JsonNullInt64  `db:"avatar_id" gql:"avatar_id"`
	CreatedAt   *time.Time     `db:"created_at" gql:"created_at"`
	UpdatedAt   *time.Time     `db:"updated_at" gql:"updated_at"`
}

func (e *Employee) PkField() *Field {
	return &Field{
		Name: "id",
		Type: BigSerial,
	}
}

func (e *Employee) Table() string {
	return "employees"
}

func (e *Employee) Pk() interface{} {
	return e.Id
}
