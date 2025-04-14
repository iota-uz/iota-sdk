package ws

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type Connection struct {
	conn     *websocket.Conn
	userID   uint
	session  *session.Session
	channels Set[string]
	mu       sync.RWMutex
	hub      *Hub
	ctx      map[string]any
}

var _ Connectioner = (*Connection)(nil)
var _ io.Closer = (*Connection)(nil)

func (c *Connection) UserID() uint {
	return c.userID
}

func (c *Connection) Session() *session.Session {
	return c.session
}

func (c *Connection) Channels() Set[string] {
	return c.channels
}

func (c *Connection) SetContext(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ctx[key] = value
}

func (c *Connection) GetContext(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.ctx[key]
	return value, ok
}

func (c *Connection) Close() error {
	return c.conn.Close()
}

func (c *Connection) SendMessage(message []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, message)
}

func (c *Connection) Subscribe(channel string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.channels[channel] = struct{}{}

	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()

	if c.hub.channelConnections[channel] == nil {
		c.hub.channelConnections[channel] = make(Set[*Connection])
	}
	c.hub.channelConnections[channel][c] = struct{}{}
}

func (c *Connection) Unsubscribe(channel string) {
	c.mu.Lock()
	delete(c.channels, channel)
	c.mu.Unlock()

	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()

	if conns, ok := c.hub.channelConnections[channel]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(c.hub.channelConnections, channel)
		}
	}
}

type Hub struct {
	upgrader           websocket.Upgrader
	connections        Set[*Connection]
	userConnections    map[uint]Set[*Connection]
	channelConnections map[string]Set[*Connection]
	mu                 sync.RWMutex
	log                *logrus.Logger
}

var _ Huber = (*Hub)(nil)

func NewHub() *Hub {
	conf := configuration.Use()
	return &Hub{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		connections:        make(Set[*Connection]),
		userConnections:    make(map[uint]Set[*Connection]),
		channelConnections: make(map[string]Set[*Connection]),
		log:                conf.Logger(),
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Printf("Error upgrading connection: %v", err)
		return
	}

	var userID uint
	if user, ok := r.Context().Value(constants.UserKey).(user.User); ok {
		userID = user.ID()
	}

	wsConn := &Connection{
		conn:     conn,
		channels: make(Set[string]),
		hub:      h,
		ctx:      make(map[string]any),
	}

	h.mu.Lock()
	h.connections[wsConn] = struct{}{}
	if userID != 0 {
		if h.userConnections[userID] == nil {
			h.userConnections[userID] = make(Set[*Connection])
		}
		h.userConnections[userID][wsConn] = struct{}{}
	}
	h.mu.Unlock()

	go h.readPump(wsConn)
}

func (h *Hub) readPump(conn *Connection) {
	defer func() {
		h.removeConnection(conn)
		_ = conn.Close()
	}()

	for {
		_, message, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.log.Printf("WebSocket error: %v", err)
			}
			break
		}

		if err := h.handleMessage(conn, message); err != nil {
			h.log.Printf("Error handling message: %v", err)
			break
		}
	}
}

func (h *Hub) removeConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.connections, conn)
	if userConns, ok := h.userConnections[conn.userID]; ok {
		delete(userConns, conn)
		if len(userConns) == 0 {
			delete(h.userConnections, conn.userID)
		}
	}
	for channel, conns := range h.channelConnections {
		if _, ok := conns[conn]; ok {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(h.channelConnections, channel)
			}
		}
	}
}

type SubscriptionMessage struct {
	Subscribe   string `json:"subscribe,omitempty"`
	Unsubscribe string `json:"unsubscribe,omitempty"`
}

func (h *Hub) handleMessage(conn *Connection, message []byte) error {
	var subMsg SubscriptionMessage
	if err := json.Unmarshal(message, &subMsg); err == nil {
		if subMsg.Subscribe != "" {
			conn.Subscribe(subMsg.Subscribe)
			return nil
		}
		if subMsg.Unsubscribe != "" {
			conn.Unsubscribe(subMsg.Unsubscribe)
			return nil
		}
	}
	return nil
}

func (h *Hub) BroadcastToAll(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.connections {
		if err := conn.SendMessage(message); err != nil {
			h.log.Printf("Error broadcasting message: %v", err)
		}
	}
}

func (h *Hub) BroadcastToUser(userID uint, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if userConns, ok := h.userConnections[userID]; ok {
		for conn := range userConns {
			if err := conn.SendMessage(message); err != nil {
				h.log.Printf("Error broadcasting to user %d: %v", userID, err)
			}
		}
	}
}

func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if channelConns, ok := h.channelConnections[channel]; ok {
		for conn := range channelConns {
			if err := conn.SendMessage(message); err != nil {
				h.log.Printf("Error broadcasting to channel %s: %v", channel, err)
			}
		}
	}
}

func (h *Hub) ConnectionsInChannel(channel string) []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if channelConns, ok := h.channelConnections[channel]; ok {
		connections := make([]*Connection, 0, len(channelConns))
		for conn := range channelConns {
			connections = append(connections, conn)
		}
		return connections
	}
	return []*Connection{}
}

func (h *Hub) ConnectionsAll() []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	connections := make([]*Connection, 0, len(h.connections))
	for conn := range h.connections {
		connections = append(connections, conn)
	}
	return connections
}
