package project

import (
	"time"
)

type Project struct {
	ID   uint
	Name string
	// Counterparty
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
