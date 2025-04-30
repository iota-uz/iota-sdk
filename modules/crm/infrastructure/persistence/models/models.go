package models

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

type Client struct {
	ID          uint
	FirstName   string
	LastName    sql.NullString
	MiddleName  sql.NullString
	PhoneNumber sql.NullString
	Address     sql.NullString
	Email       sql.NullString
	DateOfBirth sql.NullTime
	Gender      sql.NullString
	PassportID  sql.NullString // UUID reference to passports table
	Pin         sql.NullString
	Comments    sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ClientContact struct {
	ID           uint
	ClientID     uint
	ContactType  string
	ContactValue string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Chat struct {
	ID            uint
	ClientID      uint
	LastMessageAt sql.NullTime
	CreatedAt     time.Time
}

type ChatMember struct {
	ID              string
	ChatID          uint
	UserID          sql.NullInt32
	ClientID        sql.NullInt32
	ClientContactID sql.NullInt32
	Transport       string
	TransportMeta   *TransportMeta
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Message struct {
	ID        uint
	ChatID    uint
	Message   string
	ReadAt    sql.NullTime
	SenderID  string
	SentAt    sql.NullTime
	CreatedAt time.Time
}

func NewTransportMeta(value any) *TransportMeta {
	return &TransportMeta{value: value}
}

var _ sql.Scanner = &TransportMeta{}

type TransportMeta struct {
	value any
}

func (tm *TransportMeta) Interface() any {
	return tm.value
}

func (tm *TransportMeta) Value() (driver.Value, error) {
	if tm.value == nil {
		return nil, nil
	}
	return driver.Value(tm.value), nil
}

func (tm *TransportMeta) Scan(value any) error {
	if value == nil {
		tm.value = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		tm.value = string(v)
	case string:
		tm.value = v
	default:
		return fmt.Errorf("unsupported type: %T", v)
	}
	return nil
}

type TelegramMeta struct {
	ChatID   int64  `json:"chat_id"`
	Username string `json:"username"`
	Phone    string `json:"phone"`
}

type WhatsAppMeta struct {
	Phone string `json:"phone"`
}

type InstagramMeta struct {
	Username string `json:"username"`
}

type EmailMeta struct {
	Email string `json:"email"`
}

type PhoneMeta struct {
	Phone string `json:"phone"`
}

type SMSMeta struct {
	Phone string `json:"phone"`
}

// TODO: store IP address & user agent
type WebsiteMeta struct {
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type MessageTemplate struct {
	ID        uint
	Template  string
	CreatedAt time.Time
}
