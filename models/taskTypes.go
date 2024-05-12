package models

import "time"

type TaskType struct {
	Id          int64          `db:"id" gql:"id"`
	Icon        JsonNullString `db:"icon" gql:"icon"`
	Name        string         `db:"name" gql:"name"`
	Description JsonNullString `db:"description" gql:"description"`
	CreatedAt   *time.Time     `db:"created_at" gql:"created_at"`
	UpdatedAt   *time.Time     `db:"updated_at" gql:"updated_at"`
}
