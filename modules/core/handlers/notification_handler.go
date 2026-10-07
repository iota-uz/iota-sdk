package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type NotificationHandler struct {
	pool   *pgxpool.Pool
	router *services.NotificationRoutingService
	logger *logrus.Logger
}

func NewNotificationHandler(pool *pgxpool.Pool, router *services.NotificationRoutingService, logger *logrus.Logger) *NotificationHandler {
	return &NotificationHandler{pool: pool, router: router, logger: logger}
}

func (h *NotificationHandler) OnUserCreated(event *user.CreatedEvent) {
	if event == nil || event.Result == nil {
		return
	}
	u := event.Result
	parent := composables.WithPool(context.Background(), h.pool)
	if event.Context() != nil && event.Context().Value(constants.TxKey) != nil {
		parent = event.Context()
	}
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	ctx = composables.WithTenantID(ctx, u.TenantID())
	err := h.publish(ctx, notifications.Event{
		Key: "core.user.created.v1", ID: fmt.Sprintf("user-created:%d", u.ID()), TenantID: u.TenantID(),
		Data: map[string]string{"user_id": fmt.Sprint(u.ID()), "name": strings.TrimSpace(u.FirstName() + " " + u.LastName())},
	})
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", u.TenantID()).WithField("user_id", u.ID()).Error("failed to persist user-created notifications")
	}
}

func (h *NotificationHandler) publish(ctx context.Context, event notifications.Event) error {
	if tx, ok := ctx.Value(constants.TxKey).(pgx.Tx); ok {
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		rollback := func() error {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			return savepoint.Rollback(cleanup)
		}
		defer func() { _ = rollback() }()
		if _, err := h.router.Publish(composables.WithTx(ctx, savepoint), event); err != nil {
			return errors.Join(err, rollback())
		}
		return savepoint.Commit(ctx)
	}
	_, err := h.router.Publish(ctx, event)
	return err
}
