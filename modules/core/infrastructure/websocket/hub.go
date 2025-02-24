package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
)

// ConnectionContext holds per-connection metadata
type ConnectionContext struct {
	UserID    user.UserID
	SessionID user.SessionID
	Channels  map[string]bool
	Conn      *websocket.Conn
}

// Message represents a WebSocket message structure
type Message struct {
	Type    string      `json:"type"`
	Channel string      `json:"channel,omitempty"`
	Data    interface{} `json:"data"`
}

type AuthService interface {
	Authorize(ctx context.Context, token user.SessionID) (*user.Session, error)
}

// Hub manages WebSocket connections and channels
type Hub struct {
	authService AuthService
	upgrader    websocket.Upgrader
	connections map[*ConnectionContext]bool
	channels    map[string]map[*ConnectionContext]bool
	mu          sync.RWMutex
}

// NewHub creates a new Hub instance
func NewHub(authService AuthService) *Hub {
	return &Hub{
		authService: authService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Consider implementing proper origin checking
			},
		},
		connections: make(map[*ConnectionContext]bool),
		channels:    make(map[string]map[*ConnectionContext]bool),
	}
}

// ServeHTTP handles WebSocket connection upgrades
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}
	token, err := middleware.GetToken(r)
	if err != nil {
		log.Printf("Error getting token: %v", err)
		return
	}
	sess, err := h.authService.Authorize(r.Context(), token)
	if err != nil {
		log.Printf("Error authorizing connection: %v", err)
		return
	}

	ctx := &ConnectionContext{
		UserID:    sess.UserID,
		SessionID: sess.Token,
		Channels:  make(map[string]bool),
		Conn:      conn,
	}

	h.mu.Lock()
	h.connections[ctx] = true
	h.mu.Unlock()

	go h.readPump(ctx)
}

// SubscribeToChannel adds a connection to a channel
func (h *Hub) SubscribeToChannel(ctx *ConnectionContext, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*ConnectionContext]bool)
	}
	h.channels[channel][ctx] = true
	ctx.Channels[channel] = true
}

// UnsubscribeFromChannel removes a connection from a channel
func (h *Hub) UnsubscribeFromChannel(ctx *ConnectionContext, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.channels[channel] != nil {
		delete(h.channels[channel], ctx)
		delete(ctx.Channels, channel)
	}
}

// BroadcastToUser sends a message to a specific user
func (h *Hub) BroadcastToUser(userID user.UserID, message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ctx := range h.connections {
		if ctx.UserID == userID {
			h.sendMessage(ctx, message)
		}
	}
}

// BroadcastToChannel sends a message to all connections in a channel
func (h *Hub) BroadcastToChannel(channel string, message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if connections, ok := h.channels[channel]; ok {
		for ctx := range connections {
			h.sendMessage(ctx, message)
		}
	}
}

// BroadcastToAll sends a message to all active connections
func (h *Hub) BroadcastToAll(message interface{}) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for ctx := range h.connections {
		h.sendMessage(ctx, message)
	}
}

// GetConnectionsInChannel returns all connections in a channel
func (h *Hub) GetConnectionsInChannel(channel string) []*ConnectionContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var connections []*ConnectionContext
	if channelConns, ok := h.channels[channel]; ok {
		for ctx := range channelConns {
			connections = append(connections, ctx)
		}
	}
	return connections
}

// GetAllConnections returns all active connections
func (h *Hub) GetAllConnections() []*ConnectionContext {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var connections []*ConnectionContext
	for ctx := range h.connections {
		connections = append(connections, ctx)
	}
	return connections
}

// sendMessage sends a message to a specific connection
func (h *Hub) sendMessage(ctx *ConnectionContext, message interface{}) {
	msg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling message: %v", err)
		return
	}

	if err := ctx.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		log.Printf("Error sending message: %v", err)
		h.removeConnection(ctx)
	}
}

// removeConnection cleanly removes a connection and its subscriptions
func (h *Hub) removeConnection(ctx *ConnectionContext) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from all channels
	for channel := range ctx.Channels {
		if h.channels[channel] != nil {
			delete(h.channels[channel], ctx)
		}
	}

	// Remove from connections
	delete(h.connections, ctx)
	ctx.Conn.Close()
}

// readPump handles incoming messages
func (h *Hub) readPump(ctx *ConnectionContext) {
	defer h.removeConnection(ctx)

	for {
		_, message, err := ctx.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Error unmarshaling message: %v", err)
			continue
		}

		// Handle different message types
		switch msg.Type {
		case "subscribe":
			h.SubscribeToChannel(ctx, msg.Channel)
		case "unsubscribe":
			h.UnsubscribeFromChannel(ctx, msg.Channel)
			// Add other message type handlers as needed
		}
	}
}
