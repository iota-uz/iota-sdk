package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
)

type Chat interface {
	ID() uint
	Client() client.Client
	Messages() []message.Message
	AddMessages(messages ...message.Message) Chat
	SendMessage(msg string, userID uint) Chat
	CreatedAt() time.Time
}
