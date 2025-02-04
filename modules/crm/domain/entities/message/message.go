package message

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

type Sender interface {
	ID() uint
	IsClient() bool
	IsUser() bool
	FirstName() string
	LastName() string
}

type Message interface {
	ID() uint
	ChatID() uint
	Message() string
	Sender() Sender
	IsRead() bool
	MarkAsRead() Message
	ReadAt() *time.Time
	Attachments() []*upload.Upload
	CreatedAt() time.Time
}
