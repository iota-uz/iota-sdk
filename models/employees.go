package models

import "time"

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
	Id         int64          `db:"id" gql:"id"`
	FirstName  string         `db:"first_name" gql:"first_name"`
	LastName   string         `db:"last_name" gql:"last_name"`
	MiddleName JsonNullString `db:"middle_name" gql:"middle_name"`
	Email      string         `db:"email" gql:"email"`
	Phone      JsonNullString `db:"phone" gql:"phone"`
	Salary     float64        `db:"salary" gql:"salary"`
	HourlyRate float64        `db:"hourly_rate" gql:"hourly_rate"`
	PositionId int64          `db:"position_id" gql:"position_id"`
	Position   *Position      `db:"position_id" gql:"position" belongs_to:"id"`
	AvatarId   JsonNullInt64  `db:"avatar_id" gql:"avatar_id"`
	CreatedAt  *time.Time     `db:"created_at" gql:"created_at"`
	UpdatedAt  *time.Time     `db:"updated_at" gql:"updated_at"`
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
