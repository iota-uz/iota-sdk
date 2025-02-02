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
	ID        uint
	ClientID  uint
	CreatedAt time.Time
}

type Message struct {
	ID             uint
	CreatedAt      time.Time
	ChatID         uint
	Message        string
	SenderUserID   sql.NullInt64
	SenderClientID sql.NullInt64
	IsActive       bool
}

type MessageTemplate struct {
	ID        uint
	Template  string
	CreatedAt time.Time
}
