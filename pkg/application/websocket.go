package application

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/realtime"
	"github.com/iota-uz/iota-sdk/pkg/realtime/memory"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/iota-uz/iota-sdk/pkg/ws"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

const (
	ChannelAuthenticated string = "authenticated"
)

type HuberOptions struct {
	Backend        realtime.Backend
	Pool           *pgxpool.Pool
	Bundle         *i18n.Bundle
	Logger         *logrus.Logger
	CheckOrigin    func(r *http.Request) bool
	UserRepository user.Repository
}

type Connection interface {
	ws.Connectioner
	User() user.User
}

type WsCallback func(ctx context.Context, conn Connection) error

type Huber interface {
	http.Handler
	Publish(context.Context, realtime.Envelope) error
	Start(context.Context) error
	Close() error
	Counters() realtime.Counters
	ForEach(channel string, f WsCallback) error
	// ConnectionCount returns the current number of active WebSocket connections.
	ConnectionCount() int
}

func NewHub(opts *HuberOptions) Huber {
	if opts.Backend == nil {
		opts.Backend = memory.New()
	}
	if opts.Logger == nil {
		opts.Logger = logrus.New()
	}
	appHub := &huber{
		backend:         opts.Backend,
		seen:            make(map[uuid.UUID]struct{}),
		bundle:          opts.Bundle,
		pool:            opts.Pool,
		logger:          opts.Logger,
		userRepo:        opts.UserRepository,
		connectionsMeta: make(map[*ws.Connection]*MetaInfo),
	}
	hub := ws.NewHub(&ws.HubOptions{
		Logger:       opts.Logger,
		CheckOrigin:  opts.CheckOrigin,
		OnConnect:    appHub.onConnect,
		OnDisconnect: appHub.onDisconnect,
	})
	appHub.hub = hub
	return appHub
}

type MetaInfo struct {
	UserID   uint
	TenantID uuid.UUID
}

type huber struct {
	backend    realtime.Backend
	seenMu     sync.Mutex
	seen       map[uuid.UUID]struct{}
	recent     []uuid.UUID
	cancel     context.CancelFunc
	published  atomic.Uint64
	received   atomic.Uint64
	duplicates atomic.Uint64
	failed     atomic.Uint64
	oversize   atomic.Uint64

	hub               ws.Huber
	bundle            *i18n.Bundle
	pool              *pgxpool.Pool
	logger            *logrus.Logger
	connectionsMetaMu sync.RWMutex
	connectionsMeta   map[*ws.Connection]*MetaInfo
	userRepo          user.Repository
}

func (h *huber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeHTTP(w, r)
}

// ConnectionCount returns the current number of active WebSocket connections.
func (h *huber) ConnectionCount() int {
	return h.hub.ConnectionCount()
}

func (h *huber) onConnect(r *http.Request, hub *ws.Hub, conn *ws.Connection) error {
	meta := &MetaInfo{}
	usr, err := composables.UseUser(r.Context())
	if err != nil {
		// Allow unauthenticated connections - they can still receive public broadcasts
		h.setConnectionMeta(conn, meta)
		return nil //nolint:nilerr // Intentionally ignore auth error for public connections
	}
	meta.UserID = usr.ID()
	meta.TenantID = usr.TenantID()
	h.hub.JoinChannel(ChannelAuthenticated, conn)
	h.hub.JoinChannel(realtime.UserChannel(usr.TenantID(), usr.ID()), conn)
	h.setConnectionMeta(conn, meta)
	return nil
}

func (h *huber) onDisconnect(conn *ws.Connection) {
	h.deleteConnectionMeta(conn)
}

func (h *huber) setConnectionMeta(conn *ws.Connection, meta *MetaInfo) {
	h.connectionsMetaMu.Lock()
	defer h.connectionsMetaMu.Unlock()
	h.connectionsMeta[conn] = meta
}

func (h *huber) getConnectionMeta(conn *ws.Connection) (*MetaInfo, bool) {
	h.connectionsMetaMu.RLock()
	defer h.connectionsMetaMu.RUnlock()
	meta, ok := h.connectionsMeta[conn]
	return meta, ok
}

func (h *huber) deleteConnectionMeta(conn *ws.Connection) {
	h.connectionsMetaMu.Lock()
	defer h.connectionsMetaMu.Unlock()
	delete(h.connectionsMeta, conn)
}

func (h *huber) buildContext() context.Context {
	ctx := context.WithValue(
		context.Background(),
		constants.LoggerKey,
		h.logger,
	)
	return composables.WithPool(ctx, h.pool)
}

func MustParseURL(rawURL string) *url.URL {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse URL %s: %v", rawURL, err))
	}
	return parsedURL
}

func (h *huber) ForEach(channel string, f WsCallback) error {
	ctx := h.buildContext()

	// Get connections for the specific channel
	connections := h.hub.ConnectionsInChannel(channel)

	for _, conn := range connections {
		meta, ok := h.getConnectionMeta(conn)
		if !ok {
			h.logger.Error("connection meta not found")
			continue
		}
		userCtx := composables.WithTenantID(ctx, meta.TenantID)
		usr, err := h.userRepo.GetByID(userCtx, meta.UserID)
		if err != nil {
			h.logger.WithError(err).Error("failed to get user by ID")
			continue
		}
		localizer := i18n.NewLocalizer(h.bundle, string(usr.UILanguage()))
		connCtx := intl.WithLocalizer(composables.WithUser(userCtx, usr), localizer)
		connCtx = composables.WithPageCtx(connCtx, types.NewPageContext(language.English, MustParseURL("/"), localizer))
		if err := f(connCtx, &connection{
			user: usr,
			conn: conn,
		}); err != nil {
			return err
		}
	}
	return nil
}

type connection struct {
	user user.User
	conn ws.Connectioner
}

func (c *connection) SendMessage(message []byte) error {
	return c.conn.SendMessage(message)
}

func (c *connection) Close() error {
	return c.conn.Close()
}

func (c *connection) User() user.User {
	return c.user
}

func (c *connection) Connectioner() ws.Connectioner {
	return c.conn
}

func (h *huber) Start(ctx context.Context) error {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	if h.cancel != nil {
		return nil
	}
	subscriberCtx, cancel := context.WithCancel(ctx)
	if err := h.backend.Subscribe(subscriberCtx, h.receive); err != nil {
		cancel()
		return err
	}
	h.cancel = cancel
	return nil
}
func (h *huber) Close() error {
	h.seenMu.Lock()
	if h.cancel != nil {
		h.cancel()
	}
	h.seenMu.Unlock()
	for _, conn := range h.hub.ConnectionsAll() {
		_ = conn.Close()
	}
	return h.backend.Close()
}
func (h *huber) Publish(ctx context.Context, e realtime.Envelope) error {
	if err := e.Validate(); err != nil {
		if err == realtime.ErrOversize {
			h.oversize.Add(1)
			h.logger.WithField("event_id", e.ID).Warn("realtime oversize publication rejected")
		}
		return err
	}
	if err := h.backend.Publish(ctx, e); err != nil {
		h.failed.Add(1)
		h.logger.WithError(err).WithField("event_id", e.ID).Error("realtime publish failed")
		return err
	}
	h.published.Add(1)
	h.logger.WithFields(logrus.Fields{"event_id": e.ID, "tenant_id": e.TenantID, "channel": e.Channel}).Debug("realtime published")
	return nil
}
func (h *huber) receive(ctx context.Context, e realtime.Envelope) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.Validate(); err != nil {
		return err
	}
	h.seenMu.Lock()
	if _, ok := h.seen[e.ID]; ok {
		h.seenMu.Unlock()
		h.duplicates.Add(1)
		h.logger.WithField("event_id", e.ID).Debug("realtime duplicate suppressed")
		return nil
	}
	if len(h.recent) >= 4096 {
		delete(h.seen, h.recent[0])
		h.recent = h.recent[1:]
	}
	h.seen[e.ID] = struct{}{}
	h.recent = append(h.recent, e.ID)
	h.seenMu.Unlock()
	h.received.Add(1)
	h.logger.WithFields(logrus.Fields{"event_id": e.ID, "tenant_id": e.TenantID, "channel": e.Channel}).Debug("realtime received")
	for _, conn := range h.hub.ConnectionsInChannel(e.Channel) {
		if err := ctx.Err(); err != nil {
			return err
		}
		meta, ok := h.getConnectionMeta(conn)
		if !ok || meta.TenantID != e.TenantID {
			continue
		}
		if err := conn.SendMessageContext(ctx, e.Payload); err != nil {
			h.failed.Add(1)
			h.logger.WithError(err).WithField("event_id", e.ID).Warn("realtime local delivery failed")
		}
	}
	return nil
}

func (h *huber) Counters() realtime.Counters {
	c := realtime.Counters{Published: h.published.Load(), Received: h.received.Load(), Duplicate: h.duplicates.Load(), Oversize: h.oversize.Load(), DeliveryFailure: h.failed.Load()}
	if backend, ok := h.backend.(realtime.Observable); ok {
		bc := backend.Counters()
		c.Reconnect = bc.Reconnect
		c.Oversize += bc.Oversize
		c.DeliveryFailure += bc.DeliveryFailure
	}
	return c
}

func CanReceiveTenantUpdates(u user.User, tenant uuid.UUID, required permission.Permission) bool {
	return u != nil && tenant != uuid.Nil && u.TenantID() == tenant && !u.IsBlocked() && u.Status() == user.StatusActive && u.Type() == user.TypeUser && required != nil && u.Can(required)
}
