package project

import (
	"time"
)

type Project struct {
	Id          uint
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateDTO struct {
	Name        string `validate:"required"`
	Description string
}

type UpdateDTO struct {
	Name        string
	Description string
}
