package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

type Chat interface {
	ID() uint
	ClientID() uint
	Messages() []Message
	AddMessage(content string, sender Sender, attachments ...*upload.Upload) (Message, error)
	UnreadMessages() int
	MarkAllAsRead()
	LastMessage() (Message, error)
	LastMessageAt() *time.Time
	CreatedAt() time.Time
}

type Sender interface {
	ID() uint
	IsClient() bool
	IsUser() bool
	FirstName() string
	LastName() string
}

type Message interface {
	ID() uint
	Message() string
	Sender() Sender
	IsRead() bool
	MarkAsRead()
	ReadAt() *time.Time
	Attachments() []*upload.Upload
	CreatedAt() time.Time
}
