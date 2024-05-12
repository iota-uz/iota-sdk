package models

import (
	"time"
)

type Role struct {
	Id          int64          `gql:"id" db:"id"`
	Name        string         `gql:"name" db:"name"`
	Description JsonNullString `gql:"description" db:"description"`
	CreatedAt   *time.Time     `gql:"created_at" db:"created_at"`
	UpdatedAt   *time.Time     `gql:"updated_at" db:"updated_at"`
}

func (r *Role) Validate() []*ValidationError {
	var errs []*ValidationError
	if r.Name == "" {
		errs = append(errs, NewValidationError("name", "name is required"))
	}
	return errs
}
