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

func (t *TaskType) PkField() *Field {
	return &Field{
		Name: "id",
		Type: BigSerial,
	}
}

func (t *TaskType) Table() string {
	return "task_types"
}

func (t *TaskType) Pk() interface{} {
	return t.Id
}
