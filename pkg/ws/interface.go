package ws

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"

type Set[A comparable] map[A]struct{}

type Connectioner interface {
	UserID() uint
	Session() *session.Session

	Channels() Set[string]

	SendMessage(message []byte) error

	Subscribe(channel string)
	Unsubscribe(channel string)

	SetContext(key string, value any)
	GetContext(key string) (any, bool)
}

type Huber interface {
	BroadcastToAll(message []byte)
	BroadcastToUser(userID uint, message []byte)
	BroadcastToChannel(channel string, message []byte)

	ConnectionsInChannel(channel string) []*Connection
	ConnectionsAll() []*Connection
}
