package models

import (
	"database/sql"
	"time"
)

type Client struct {
	ID          uint
	FirstName   string
	LastName    string
	MiddleName  sql.NullString
	PhoneNumber string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Chat struct {
	ID        string
	CreatedAt time.Time
	ClientID  int64
}

type Message struct {
	ID             string
	CreatedAt      time.Time
	ChatID         string
	Message        string
	SenderUserID   sql.NullString
	SenderClientID sql.NullInt64
	IsActive       bool
}
