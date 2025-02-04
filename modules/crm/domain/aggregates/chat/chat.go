package chat

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
)

type Chat interface {
	ID() uint
	Client() client.Client
	CreatedAt() time.Time
}
