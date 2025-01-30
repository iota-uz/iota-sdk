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
	CreatedAt() time.Time
}
