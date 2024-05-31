package models

import "time"

type Expense struct {
	Id         int64 `gorm:"primaryKey"`
	Amount     float64
	CategoryId int64
	Category   *ExpenseCategory `gorm:"foreignKey:CategoryId"`
	Date       *time.Time
	CreatedAt  *time.Time
	UpdatedAt  *time.Time
}
