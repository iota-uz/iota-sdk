package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
)

func New(client client.Client, messages []message.Message) Chat {
	return &chat{
		id:        0,
		client:    client,
		messages:  messages,
		createdAt: time.Now(),
	}
}

func NewWithID(
	id uint,
	client client.Client,
	messages []message.Message,
	createdAt time.Time,
) Chat {
	return &chat{
		id:        id,
		client:    client,
		messages:  messages,
		createdAt: createdAt,
	}
}

type chat struct {
	id        uint
	client    client.Client
	messages  []message.Message
	createdAt time.Time
}

func (c *chat) ID() uint {
	return c.id
}

func (c *chat) Client() client.Client {
	return c.client
}

func (c *chat) Messages() []message.Message {
	return c.messages
}

func (c *chat) AddMessages(messages ...message.Message) Chat {
	return &chat{
		id:        c.id,
		client:    c.client,
		messages:  append(c.messages, messages...),
		createdAt: c.createdAt,
	}
}

func (c *chat) SendMessage(msg string, userID uint) Chat {
	sender := message.NewUserSender(userID)
	return c.AddMessages(message.NewMessage(c.id, msg, sender))
}

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}
