package models

import "time"

type Expense struct {
	Id         int64            `db:"id" gql:"id"`
	Amount     float64          `db:"amount" gql:"amount"`
	CategoryId int64            `db:"category_id" gql:"category_id"`
	Category   *ExpenseCategory `db:"category_id" gql:"category" belongs_to:"id"`
	Date       *time.Time       `db:"date" gql:"date"`
	CreatedAt  *time.Time       `db:"created_at" gql:"created_at"`
	UpdatedAt  *time.Time       `db:"updated_at" gql:"updated_at"`
}

func (e *Expense) PkField() *Field {
	return &Field{
		Name: "id",
		Type: BigSerial,
	}
}

func (e *Expense) Table() string {
	return "expenses"
}

func (e *Expense) Pk() interface{} {
	return e.Id
}
