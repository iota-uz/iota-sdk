package ws

import (
	"io"
	"net/http"
)

type Set[A comparable] map[A]struct{}
type EventType int

const (
	EventTypeMessage EventType = iota
	EventTypeJoin
	EventTypeLeave
	EventTypeClose
)

type Connectioner interface {
	io.Closer
	SendMessage(message []byte) error
}

type Huber interface {
	http.Handler

	BroadcastToAll(message []byte)
	BroadcastToChannel(channel string, message []byte)

	// On registers a handler for the given event type. An event type can have multiple handlers.
	On(eventType EventType, handler func(conn *Connection, message []byte))

	JoinChannel(channel string, conn *Connection)
	LeaveChannel(channel string, conn *Connection)

	ConnectionsInChannel(channel string) []*Connection
	ConnectionsAll() []*Connection
}
