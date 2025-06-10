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
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ClientHandler struct {
	pool          *pgxpool.Pool
	publisher     eventbus.EventBus
	chatService   *crmservices.ChatService
	tenantService *services.TenantService
}

func RegisterClientHandler(app application.Application) *ClientHandler {
	handler := &ClientHandler{
		pool:          app.DB(),
		publisher:     app.EventPublisher(),
		chatService:   app.Service(crmservices.ChatService{}).(*crmservices.ChatService),
		tenantService: app.Service(services.TenantService{}).(*services.TenantService),
	}
	app.EventPublisher().Subscribe(handler.onCreated)
	return handler
}

func (h *ClientHandler) createTenantContext(tenantID uuid.UUID) context.Context {
	ctx := context.Background()
	ctxWithDb := composables.WithPool(ctx, h.pool)
	logger := configuration.Use().Logger()

	tenant, err := h.tenantService.GetByID(ctxWithDb, tenantID)
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

func (h *ClientHandler) onCreated(event *client.CreatedEvent) {
	tenantID := event.Result.TenantID()
	logger := configuration.Use().Logger()

	// Validate tenant exists before creating chat
	ctxWithDb := composables.WithPool(context.Background(), h.pool)
	if _, err := h.tenantService.GetByID(ctxWithDb, tenantID); err != nil {
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
