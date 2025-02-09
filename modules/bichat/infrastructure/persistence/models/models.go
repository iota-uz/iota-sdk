package models

import (
	"time"
)

type Prompt struct {
	ID          string
	Title       string
	Description string
	Prompt      string
	CreatedAt   time.Time
}

type Dialogue struct {
	ID        uint
	UserID    uint
	Label     string
	Messages  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
