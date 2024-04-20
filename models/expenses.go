package models

import "time"

type Expense struct {
	Id         int64      `db:"id" gql:"id"`
	Amount     float64    `db:"amount" gql:"amount"`
	CategoryId int64      `db:"category_id" gql:"category_id"`
	CreatedAt  *time.Time `db:"created_at" gql:"created_at"`
	UpdatedAt  *time.Time `db:"updated_at" gql:"updated_at"`
}
