package models

import (
	"time"
)

type Role struct {
	Id          int64
	Name        string
	Description JsonNullString
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}
