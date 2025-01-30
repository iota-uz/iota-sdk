package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
)

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

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}
