package message

import "time"

type Sender interface {
	ID() uint
	IsClient() bool
	IsUser() bool
}

type Message interface {
	ID() uint
	ChatID() uint
	Message() string
	Sender() Sender
	IsRead() bool
	CreatedAt() time.Time
}
