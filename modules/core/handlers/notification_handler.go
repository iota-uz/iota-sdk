package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = composables.WithTenantID(composables.WithPool(ctx, h.pool), u.TenantID())
	_, err := h.router.Publish(ctx, notifications.Event{
		Key: "core.user.created.v1", ID: fmt.Sprintf("user-created:%d", u.ID()), TenantID: u.TenantID(),
		Data: map[string]string{"user_id": fmt.Sprint(u.ID()), "name": strings.TrimSpace(u.FirstName() + " " + u.LastName())},
	})
	if err != nil {
		h.logger.WithError(err).WithField("tenant_id", u.TenantID()).WithField("user_id", u.ID()).Error("failed to persist user-created notifications")
	}
}
