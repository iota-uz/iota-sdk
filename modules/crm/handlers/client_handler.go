// Package handlers provides this package.
package handlers

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmservices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// ClientHandler subscribes to client lifecycle events and creates a chat
// aggregate every time a new client is persisted. It is a pure struct
// registered via composition.ProvideFunc; the composition Hook takes care
// of Subscribe / Unsubscribe on Start / Stop.
type ClientHandler struct {
	pool          *pgxpool.Pool
	chatService   *crmservices.ChatService
	tenantService *services.TenantService
}

func NewClientHandler(
	pool *pgxpool.Pool,
	chatService *crmservices.ChatService,
	tenantService *services.TenantService,
) *ClientHandler {
	return &ClientHandler{
		pool:          pool,
		chatService:   chatService,
		tenantService: tenantService,
	}
}

func (h *ClientHandler) createTenantContext(tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	ctxWithDB := composables.WithPool(ctx, h.pool)
	logger := configuration.Use().Logger()

	tenant, err := h.tenantService.GetByID(ctxWithDB, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to get tenant")
		return composables.WithPool(ctx, h.pool)
	}

	tenantComposable := &composables.Tenant{
		ID:     tenant.ID(),
		Name:   tenant.Name(),
		Domain: tenant.Domain(),
	}

	return composables.WithPool(composables.WithTenantID(ctx, tenantComposable.ID), h.pool)
}

// OnCreated is the eventbus subscriber callback. Compatible with
// eventbus.EventBus.Subscribe.
func (h *ClientHandler) OnCreated(event *client.CreatedEvent) {
	tenantID := event.Result.TenantID()
	logger := configuration.Use().Logger()

	// Validate tenant exists before creating chat
	ctxWithDB := composables.WithPool(context.Background(), h.pool)
	if _, err := h.tenantService.GetByID(ctxWithDB, tenantID); err != nil {
		logger.WithFields(logrus.Fields{
			"tenant_id": tenantID,
		}).WithError(err).Error("failed to get tenant")
		return
	}

	ctx := h.createTenantContext(tenantID)

	if _, err := h.chatService.Save(ctx, chat.New(
		event.Result.ID(),
		chat.WithTenantID(tenantID),
	)); err != nil {
		logger.WithFields(logrus.Fields{
			"tenant_id": tenantID,
			"client_id": event.Result.ID(),
		}).WithError(err).Error("failed to register client chat")
		return
	}

	logger.WithFields(logrus.Fields{
		"client_id": event.Result.ID(),
		"tenant_id": tenantID,
	}).Info("successfully created chat for client")
}
