package wsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	appletengineruntime "github.com/iota-uz/iota-sdk/pkg/appletengine/runtime"
	"github.com/sirupsen/logrus"
)

type websocketConn interface {
	WriteMessage(messageType int, data []byte) error
	ReadMessage() (messageType int, p []byte, err error)
	Close() error
}

type connEntry struct {
	appletID     string
	tenantID     string
	connectionID string
	conn         websocketConn
}

type Bridge struct {
	mu      sync.RWMutex
	logger  *logrus.Logger
	entries map[string]*connEntry
	runtime *appletengineruntime.Manager
}

func New(logger *logrus.Logger) *Bridge {
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	return &Bridge{
		logger:  logger,
		entries: make(map[string]*connEntry),
	}
}

func (b *Bridge) SetRuntimeManager(runtime *appletengineruntime.Manager) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runtime = runtime
}

func (b *Bridge) AddConnection(ctx context.Context, appletID, tenantID string, conn websocketConn) string {
	connectionID := uuid.NewString()
	entry := &connEntry{
		appletID:     appletID,
		tenantID:     tenantID,
		connectionID: connectionID,
		conn:         conn,
	}
	b.mu.Lock()
	b.entries[connectionID] = entry
	runtime := b.runtime
	b.mu.Unlock()

	if runtime != nil {
		dispatchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := runtime.DispatchWebsocketEvent(dispatchCtx, appletID, tenantID, connectionID, "open", nil); err != nil {
			b.logger.WithError(err).WithField("applet", appletID).WithField("connection_id", connectionID).Warn("failed to dispatch websocket open event to applet")
		}
	}
	return connectionID
}

func (b *Bridge) RemoveConnection(ctx context.Context, connectionID string) {
	b.mu.Lock()
	entry := b.entries[connectionID]
	if entry != nil {
		delete(b.entries, connectionID)
	}
	runtime := b.runtime
	b.mu.Unlock()

	if entry == nil || runtime == nil {
		return
	}
	dispatchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := runtime.DispatchWebsocketEvent(dispatchCtx, entry.appletID, entry.tenantID, connectionID, "close", nil); err != nil {
		b.logger.WithError(err).WithField("applet", entry.appletID).WithField("connection_id", connectionID).Warn("failed to dispatch websocket close event to applet")
	}
}

func (b *Bridge) DispatchMessage(ctx context.Context, connectionID string, message []byte) {
	b.mu.RLock()
	entry := b.entries[connectionID]
	runtime := b.runtime
	b.mu.RUnlock()

	if entry == nil || runtime == nil {
		return
	}
	dispatchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := runtime.DispatchWebsocketEvent(dispatchCtx, entry.appletID, entry.tenantID, connectionID, "message", message); err != nil {
		b.logger.WithError(err).WithField("applet", entry.appletID).WithField("connection_id", connectionID).Warn("failed to dispatch websocket message event to applet")
	}
}

func (b *Bridge) Send(appletID, connectionID string, payload any) error {
	b.mu.RLock()
	entry := b.entries[connectionID]
	b.mu.RUnlock()
	if entry == nil || entry.conn == nil {
		return fmt.Errorf("connection not found")
	}
	if entry.appletID != appletID {
		return fmt.Errorf("connection does not belong to applet %q", appletID)
	}

	var (
		messageType = 1 // websocket.TextMessage
		data        []byte
	)
	switch v := payload.(type) {
	case string:
		data = []byte(v)
	case []byte:
		messageType = 2 // websocket.BinaryMessage
		data = v
	case map[string]any:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal websocket payload: %w", err)
		}
		data = encoded
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal websocket payload: %w", err)
		}
		data = encoded
	}

	if err := entry.conn.WriteMessage(messageType, data); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}
	return nil
}

func (b *Bridge) HandleAppletEvent(w http.ResponseWriter, r *http.Request) {
	defer func() { _ = r.Body.Close() }()
	var payload struct {
		AppletID     string `json:"appletId"`
		ConnectionID string `json:"connectionId"`
		Event        string `json:"event"`
		DataBase64   string `json:"dataBase64"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if payload.Event != "send" {
		http.Error(w, "unsupported event", http.StatusBadRequest)
		return
	}
	var data any = ""
	if payload.DataBase64 != "" {
		decoded, err := base64.StdEncoding.DecodeString(payload.DataBase64)
		if err != nil {
			http.Error(w, "invalid dataBase64", http.StatusBadRequest)
			return
		}
		data = decoded
	}
	if err := b.Send(payload.AppletID, payload.ConnectionID, data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
