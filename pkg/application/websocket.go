package application

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
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
	Pool           *pgxpool.Pool
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
	ForEach(channel string, f WsCallback) error
}

func NewHub(opts *HuberOptions) Huber {
	appHub := &huber{
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
	hub             ws.Huber
	app             Application
	pool            *pgxpool.Pool
	logger          *logrus.Logger
	connectionsMeta map[*ws.Connection]*MetaInfo
	userRepo        user.Repository
}

func (h *huber) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeHTTP(w, r)
}

func (h *huber) onConnect(r *http.Request, hub *ws.Hub, conn *ws.Connection) error {
	meta := &MetaInfo{}
	usr, err := composables.UseUser(r.Context())
	if err != nil {
		// Allow unauthenticated connections - they can still receive public broadcasts
		h.connectionsMeta[conn] = meta
		return nil //nolint:nilerr // Intentionally ignore auth error for public connections
	}
	meta.UserID = usr.ID()
	meta.TenantID = usr.TenantID()
	h.hub.JoinChannel(ChannelAuthenticated, conn)
	h.hub.JoinChannel(fmt.Sprintf("user/%d", usr.ID()), conn)
	h.connectionsMeta[conn] = meta
	return nil
}

func (h *huber) onDisconnect(conn *ws.Connection) {
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

func (h *huber) ForEach(channel string, f WsCallback) error {
	ctx := h.buildContext()

	// Get connections for the specific channel
	connections := h.hub.ConnectionsInChannel(channel)

	for _, conn := range connections {
		meta, ok := h.connectionsMeta[conn]
		if !ok {
			h.logger.Error("connection meta not found")
			continue
		}
		usr, err := h.userRepo.GetByID(ctx, meta.UserID)
		if err != nil {
			h.logger.WithError(err).Error("failed to get user by ID")
			continue
		}
		localizer := i18n.NewLocalizer(h.app.Bundle(), string(usr.UILanguage()))
		connCtx := intl.WithLocalizer(ctx, localizer)
		connCtx = composables.WithPageCtx(connCtx, &types.PageContext{
			URL:       nil,
			Locale:    language.English,
			Localizer: localizer,
		})
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
