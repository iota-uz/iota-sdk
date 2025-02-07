package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
)

func New(client client.Client) Chat {
	return &chat{
		id:        0,
		client:    client,
		createdAt: time.Now(),
	}
}

func NewWithID(
	id uint,
	client client.Client,
	createdAt time.Time,
) Chat {
	return &chat{
		id:        id,
		client:    client,
		createdAt: createdAt,
	}
}

type chat struct {
	id        uint
	client    client.Client
	createdAt time.Time
}

func (c *chat) ID() uint {
	return c.id
}

func (c *chat) Client() client.Client {
	return c.client
}

func (c *chat) CreatedAt() time.Time {
	return c.createdAt
}
