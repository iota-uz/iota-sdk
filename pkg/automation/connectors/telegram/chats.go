package telegram

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChatInfo is a chat a tenant's bot received a message from.
type ChatInfo struct {
	ID       int64     `json:"id"`
	Type     string    `json:"type"` // private | group | supergroup | channel
	Title    string    `json:"title"`
	Username string    `json:"username,omitempty"`
	LastSeen time.Time `json:"last_seen"`
}

// Bounds of the known chats of a connection, kept in its inbound state (≤ 1 MiB).
const (
	maxKnownChats   = 300
	chatRewriteGap  = 6 * time.Hour
	chatCacheSize   = 4096
	inboundStateKey = "telegram"
)

type knownState struct {
	Chats []ChatInfo `json:"chats"`
}

// chatMemory throttles the writes of known chats: a chat is written when it is new, renamed, or last
// written long ago.
type chatMemory struct {
	mu   sync.Mutex
	seen map[string]ChatInfo
}

func (m *chatMemory) due(conn string, ci ChatInfo) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := conn + "/" + strconv.FormatInt(ci.ID, 10)
	prev, ok := m.seen[key]
	if ok && prev.Title == ci.Title && prev.Username == ci.Username && ci.LastSeen.Sub(prev.LastSeen) < chatRewriteGap {
		return false
	}
	if m.seen == nil || len(m.seen) >= chatCacheSize {
		m.seen = map[string]ChatInfo{}
	}
	m.seen[key] = ci
	return true
}

// rememberChat records a chat of an incoming message in the connection's inbound state, for the chat
// picker of the nodes. A failure is logged: the message is handled all the same.
func (c *Connector) rememberChat(ctx context.Context, tenantID, connectionID uuid.UUID, ci ChatInfo) {
	if c.cfg.Connections == nil || tenantID == uuid.Nil || connectionID == uuid.Nil {
		return
	}
	if !c.chats.due(connectionID.String(), ci) {
		return
	}
	err := c.cfg.Connections.UpdateInbound(ctx, tenantID, connectionID, func(state json.RawMessage) (json.RawMessage, error) {
		doc := map[string]json.RawMessage{}
		if len(state) > 0 {
			if err := json.Unmarshal(state, &doc); err != nil {
				doc = map[string]json.RawMessage{}
			}
		}
		var known knownState
		if raw, ok := doc[inboundStateKey]; ok {
			_ = json.Unmarshal(raw, &known)
		}
		replaced := false
		for i := range known.Chats {
			if known.Chats[i].ID == ci.ID {
				known.Chats[i], replaced = ci, true
			}
		}
		if !replaced {
			known.Chats = append(known.Chats, ci)
		}
		sort.SliceStable(known.Chats, func(i, j int) bool { return known.Chats[i].LastSeen.After(known.Chats[j].LastSeen) })
		if len(known.Chats) > maxKnownChats {
			known.Chats = known.Chats[:maxKnownChats]
		}
		raw, err := json.Marshal(known)
		if err != nil {
			return nil, err
		}
		doc[inboundStateKey] = raw
		return json.Marshal(doc)
	})
	if err != nil {
		c.log().Warn("telegram: a known chat was not recorded", "error", err.Error())
	}
}

// KnownChats lists the chats the connection received messages from, the most recent first, narrowed by
// query (a part of the title or @username, or the id). Only a tenant's own bot has any.
func (c *Connector) KnownChats(ctx context.Context, tenantID, connectionID uuid.UUID, query string, limit int) ([]ChatInfo, error) {
	reader, ok := c.cfg.Connections.(interface {
		InboundState(ctx context.Context, tenantID, id uuid.UUID) (json.RawMessage, error)
	})
	if !ok {
		return nil, nil
	}
	state, err := reader.InboundState(ctx, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if json.Unmarshal(state, &doc) != nil {
		return nil, nil
	}
	var known knownState
	if raw, ok := doc[inboundStateKey]; ok {
		_ = json.Unmarshal(raw, &known)
	}
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(query), "@"))
	var out []ChatInfo
	for _, ci := range known.Chats {
		if q != "" && !strings.Contains(strings.ToLower(ci.Title), q) &&
			!strings.Contains(strings.ToLower(ci.Username), q) && !strings.Contains(strconv.FormatInt(ci.ID, 10), q) {
			continue
		}
		out = append(out, ci)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastSeen.After(out[j].LastSeen) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
