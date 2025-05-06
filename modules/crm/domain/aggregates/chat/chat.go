package chat

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
)

// ---- Interfaces ----

type SenderType string

const (
	UnknownSenderType SenderType = "unknown"
	UserSenderType    SenderType = "user"
	ClientSenderType  SenderType = "client"
)

type Transport string

const (
	TelegramTransport  Transport = "telegram"
	WhatsAppTransport  Transport = "whatsapp"
	InstagramTransport Transport = "instagram"
	SMSTransport       Transport = "sms"
	EmailTransport     Transport = "email"
	PhoneTransport     Transport = "phone"
	WebsiteTransport   Transport = "website"
	OtherTransport     Transport = "other"
)

type Provider interface {
	Transport() Transport
	Send(ctx context.Context, msg Message) error
	OnReceived(callback func(msg Message) error)
}

type Chat interface {
	ID() uint
	WithID(id uint) Chat
	ClientID() uint
	Messages() []Message
	AddMessage(msg Message) Chat
	UnreadMessages() int
	MarkAllAsRead()
	Members() []Member
	AddMember(member Member) Chat
	RemoveMember(memberID uuid.UUID) Chat
	LastMessage() (Message, error)
	LastMessageAt() *time.Time
	CreatedAt() time.Time
}

type Message interface {
	ID() uint
	ChatID() uint
	Sender() Member
	Message() string
	IsRead() bool
	MarkAsRead()
	ReadAt() *time.Time
	SentAt() *time.Time
	Attachments() []upload.Upload
	CreatedAt() time.Time
}

type Member interface {
	ID() uuid.UUID
	Transport() Transport
	Sender() Sender
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

type Sender interface {
	Type() SenderType
	Transport() Transport
}

type UserSender interface {
	Sender
	UserID() uint
	FirstName() string
	LastName() string
}

type ClientSender interface {
	Sender
	ClientID() uint
	ContactID() uint
	FirstName() string
	LastName() string
}

type TelegramSender interface {
	Sender
	ChatID() int64
	Username() string
	Phone() phone.Phone
}

type WhatsAppSender interface {
	Sender
	Phone() phone.Phone
}

type InstagramSender interface {
	Sender
	Username() string
}

type SMSSender interface {
	Sender
	Phone() phone.Phone
}

type EmailSender interface {
	Sender
	Email() internet.Email
}

type PhoneSender interface {
	Sender
	Phone() phone.Phone
}

type WebsiteSender interface {
	Sender
	Phone() phone.Phone
	Email() internet.Email
}

type OtherSender interface {
	Sender
}
