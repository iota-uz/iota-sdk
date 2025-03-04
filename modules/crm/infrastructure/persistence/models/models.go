package models

import (
	"database/sql"
	"time"
)

type Client struct {
	ID          uint
	FirstName   string
	LastName    sql.NullString
	MiddleName  sql.NullString
	PhoneNumber string
	Address     sql.NullString
	Email       sql.NullString
	HourlyRate  sql.NullFloat64
	DateOfBirth sql.NullTime
	Gender      sql.NullString
	PassportID  sql.NullString // UUID reference to passports table
	Pin         sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Chat struct {
	ID            uint
	ClientID      uint
	LastMessageAt sql.NullTime
	CreatedAt     time.Time
}

type Message struct {
	ID             uint
	CreatedAt      time.Time
	ChatID         uint
	Message        string
	SenderUserID   sql.NullInt64
	SenderClientID sql.NullInt64
	ReadAt         sql.NullTime
	IsRead         bool
}

type MessageTemplate struct {
	ID        uint
	Template  string
	CreatedAt time.Time
}
