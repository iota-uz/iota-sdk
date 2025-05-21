package core

import (
	"context"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/ws"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type HuberOptions struct {
	Hub            *ws.Hub
	Pool           *pgxpool.Pool
	Logger         *logrus.Logger
	UserRepository user.Repository
}

type Connection interface {
	User() user.User
	Connectioner() ws.Connectioner
}

type WsCallback func(ctx context.Context, conn Connection) error

type Huber interface {
	ForEach(channel string, f WsCallback) error
}

func NewHuber(opts *HuberOptions) Huber {
	return &huber{
		hub:      opts.Hub,
		pool:     opts.Pool,
		logger:   opts.Logger,
		userRepo: opts.UserRepository,
	}
}

type huber struct {
	hub      ws.Huber
	pool     *pgxpool.Pool
	logger   *logrus.Logger
	userRepo user.Repository
}

func (h *huber) ForEach(cannel string, f WsCallback) error {
	ctx := context.Background()
	ctx = composables.WithPool(ctx, h.pool)
	for _, conn := range h.hub.ConnectionsAll() {
		userEntity, err := h.userRepo.GetByID(ctx, conn)
		if err != nil {
			h.logger.WithError(err).Error("failed to get user by ID")
			continue
		}
		if err := f(&connection{
			user: userEntity,
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

func (c *connection) User() user.User {
	return c.user
}

func (c *connection) Connectioner() ws.Connectioner {
	return c.conn
}
