package models

import "time"

type Expense struct {
	Id         int64            `gql:"id" gorm:"primaryKey"`
	Amount     float64          `gql:"amount"`
	CategoryId int64            `gql:"category_id"`
	Category   *ExpenseCategory `gql:"category" gorm:"foreignKey:CategoryId"`
	Date       *time.Time       `gql:"date"`
	CreatedAt  *time.Time       `gql:"created_at"`
	UpdatedAt  *time.Time       `gql:"updated_at"`
}
