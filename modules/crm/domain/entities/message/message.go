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
	IsActive() bool
	CreatedAt() time.Time
}
