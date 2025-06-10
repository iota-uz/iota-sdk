package ws

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

type HubOptions struct {
	Logger      *logrus.Logger
	CheckOrigin func(r *http.Request) bool
	OnConnect   func(r *http.Request, hub *Hub, conn *Connection) error
}

type Connection struct {
	conn *websocket.Conn
	hub  *Hub
	ctx  map[string]any
}

var _ Connectioner = (*Connection)(nil)

func (c *Connection) Close() error {
	return c.conn.Close()
}

// SendMessage sends a text message to the websocket connection
func (c *Connection) SendMessage(message []byte) error {
	return c.conn.WriteMessage(websocket.TextMessage, message)
}

type Hub struct {
	upgrader           websocket.Upgrader
	connections        Set[*Connection]
	channelConnections map[string]Set[*Connection]
	eventHandlers      map[EventType][]func(conn *Connection, message []byte)
	mu                 sync.RWMutex
	logger             *logrus.Logger
	OnConnect          func(r *http.Request, hub *Hub, conn *Connection) error
}

var _ Huber = (*Hub)(nil)

func NewHub(opts *HubOptions) *Hub {
	return &Hub{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin:     opts.CheckOrigin,
		},
		connections:        make(Set[*Connection]),
		channelConnections: make(map[string]Set[*Connection]),
		eventHandlers:      make(map[EventType][]func(conn *Connection, message []byte)),
		logger:             opts.Logger,
		OnConnect:          opts.OnConnect,
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("Error upgrading connection: %v", err)
		return
	}

	wsConn := &Connection{
		conn: conn,
		hub:  h,
		ctx:  make(map[string]any),
	}

	// Call OnConnect callback if provided
	if h.OnConnect != nil {
		if err := h.OnConnect(r, h, wsConn); err != nil {
			h.logger.Printf("Connection rejected by OnConnect callback: %v", err)
			_ = conn.Close()
			return
		}
	}

	h.mu.Lock()
	h.connections[wsConn] = struct{}{}
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
				h.logger.Printf("WebSocket error: %v", err)
			}
			break
		}

		if err := h.handleMessage(conn, message); err != nil {
			h.logger.Printf("Error handling message: %v", err)
			break
		}
	}
}

func (h *Hub) removeConnection(conn *Connection) {
	// Collect all channels this connection was in to trigger leave events
	h.mu.RLock()
	channelsCopy := make([]string, 0)
	for channel, conns := range h.channelConnections {
		if _, ok := conns[conn]; ok {
			channelsCopy = append(channelsCopy, channel)
		}
	}
	h.mu.RUnlock()

	// Trigger leave handlers for each channel
	for _, channel := range channelsCopy {
		h.mu.RLock()
		handlers, exists := h.eventHandlers[EventTypeLeave]
		h.mu.RUnlock()

		if exists {
			for _, handler := range handlers {
				handler(conn, []byte(channel))
			}
		}
	}

	// Remove connection from tracking structures
	h.mu.Lock()
	delete(h.connections, conn)
	for channel, conns := range h.channelConnections {
		if _, ok := conns[conn]; ok {
			delete(conns, conn)
			if len(conns) == 0 {
				delete(h.channelConnections, channel)
			}
		}
	}
	h.mu.Unlock()

	// Trigger close handlers
	h.mu.RLock()
	handlers, exists := h.eventHandlers[EventTypeClose]
	h.mu.RUnlock()

	if exists {
		for _, handler := range handlers {
			handler(conn, nil)
		}
	}
}

func (h *Hub) handleMessage(conn *Connection, message []byte) error {
	h.mu.RLock()
	handlers, exists := h.eventHandlers[EventTypeMessage]
	h.mu.RUnlock()

	if exists {
		for _, handler := range handlers {
			handler(conn, message)
		}
	}

	return nil
}

func (h *Hub) BroadcastToAll(message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range h.connections {
		if err := conn.SendMessage(message); err != nil {
			h.logger.Printf("Error broadcasting message: %v", err)
		}
	}
}

func (h *Hub) BroadcastToChannel(channel string, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if channelConns, ok := h.channelConnections[channel]; ok {
		for conn := range channelConns {
			if err := conn.SendMessage(message); err != nil {
				h.logger.Printf("Error broadcasting to channel %s: %v", channel, err)
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

func (h *Hub) JoinChannel(channel string, conn *Connection) {
	h.mu.Lock()
	if h.channelConnections[channel] == nil {
		h.channelConnections[channel] = make(Set[*Connection])
	}
	h.channelConnections[channel][conn] = struct{}{}
	h.mu.Unlock()

	// Trigger join handlers
	h.mu.RLock()
	handlers, exists := h.eventHandlers[EventTypeJoin]
	h.mu.RUnlock()

	if exists {
		for _, handler := range handlers {
			handler(conn, []byte(channel))
		}
	}
}

func (h *Hub) LeaveChannel(channel string, conn *Connection) {
	h.mu.Lock()
	if conns, ok := h.channelConnections[channel]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.channelConnections, channel)
		}
	}
	h.mu.Unlock()

	// Trigger leave handlers
	h.mu.RLock()
	handlers, exists := h.eventHandlers[EventTypeLeave]
	h.mu.RUnlock()

	if exists {
		for _, handler := range handlers {
			handler(conn, []byte(channel))
		}
	}
}

func (h *Hub) On(eventType EventType, handler func(conn *Connection, message []byte)) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.eventHandlers[eventType] = append(h.eventHandlers[eventType], handler)
}
