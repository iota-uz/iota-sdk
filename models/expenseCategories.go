package models

import "time"

type ExpenseCategory struct {
	Id          int64          `db:"id" gql:"id"`
	Name        string         `db:"name" gql:"name"`
	Description JsonNullString `db:"description" gql:"description"`
	Amount      float64        `db:"amount" gql:"amount"`
	CreatedAt   *time.Time     `db:"created_at" gql:"created_at"`
	UpdatedAt   *time.Time     `db:"updated_at" gql:"updated_at"`
}

func (e *ExpenseCategory) Pk() interface{} {
	return e.Id
}

func (e *ExpenseCategory) PkField() *Field {
	return &Field{
		Name: "id",
		Type: BigSerial,
	}
}

func (e *ExpenseCategory) Table() string {
	return "expense_categories"
}
