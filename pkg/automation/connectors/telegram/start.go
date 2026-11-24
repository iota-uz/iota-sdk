package telegram

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/google/uuid"
)

// StartHandler is an optional hook for `/start <payload>` with its prefix, on the platform bot and on
// tenant bots. It is implemented and registered by other modules (the Telegram linking of a user, #1349);
// the connector does not import them.
type StartHandler interface {
	Prefix() string // payload prefix, e.g. "u_"; unique, [a-z]{1,4}_
	// TenantOf resolves the tenant of the payload. Platform bot: the only source of InboundEvent tenant for /start.
	// Tenant bot: must equal the connection tenant, otherwise the update is treated as unknown.
	TenantOf(ctx context.Context, payload string) (tenantID uuid.UUID, ok bool, err error)
	// Handle is called after Verify; reply is sent to the chat. ok == false → TenantID is uuid.Nil, Handle only replies.
	// An error → 5xx to Telegram, the update is redelivered; Handle must be safe to repeat.
	Handle(ctx context.Context, u StartUpdate) (reply string, err error)
}

// StartUpdate is a `/start <payload>` a StartHandler handles.
type StartUpdate struct {
	TenantID       uuid.UUID // from TenantOf; uuid.Nil for an unknown or used payload
	ConnectionID   string    // UUID or "platform:telegram_bot"
	ChatID, UserID int64
	ChatType       string // private | group | supergroup | channel
	Username       string
	FirstName      string
	Payload        string // with prefix
	// UpdateKey names the update ("<bot id>:<update id>"): the same one on every delivery of it. A handler that
	// consumes a one-use link keeps its result under this key in the transaction that consumes the link, and
	// returns that result when it is asked again, so a redelivery after a lost result sends the reply of the
	// first run instead of "unknown or used".
	UpdateKey string
}

var prefixRe = regexp.MustCompile(`^[a-z]{1,4}_$`)

type starts struct {
	mu   sync.RWMutex
	list []StartHandler
}

// RegisterStart registers a handler of `/start <payload>`; a prefix twice is an error.
func (c *Connector) RegisterStart(h StartHandler) error {
	if h == nil {
		return fmt.Errorf("telegram: a nil start handler")
	}
	prefix := h.Prefix()
	if !prefixRe.MatchString(prefix) {
		return fmt.Errorf("telegram: start prefix %q is 1-4 lowercase letters and an underscore", prefix)
	}
	c.starts.mu.Lock()
	defer c.starts.mu.Unlock()
	for _, other := range c.starts.list {
		if other.Prefix() == prefix {
			return fmt.Errorf("telegram: start prefix %q is registered twice", prefix)
		}
	}
	c.starts.list = append(c.starts.list, h)
	return nil
}

// startHandlerFor is the handler whose prefix the payload starts with.
func (c *Connector) startHandlerFor(payload string) (StartHandler, bool) {
	c.starts.mu.RLock()
	defer c.starts.mu.RUnlock()
	for _, h := range c.starts.list {
		if p := h.Prefix(); len(payload) >= len(p) && payload[:len(p)] == p {
			return h, true
		}
	}
	return nil, false
}
