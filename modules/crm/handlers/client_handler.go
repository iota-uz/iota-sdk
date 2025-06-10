package handlers

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	crmservices "github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
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

	tenant, err := h.tenantService.GetByID(ctxWithDb, tenantID)
	if err != nil {
		log.Printf("failed to get tenant: %v", err)
		return composables.WithPool(ctx, h.pool)
	}

	tenantComposable := &composables.Tenant{
		ID:     tenant.ID(),
		Name:   tenant.Name(),
		Domain: tenant.Domain(),
	}

	return composables.WithPool(composables.WithTenant(ctx, tenantComposable), h.pool)
}

func (h *ClientHandler) onCreated(event *client.CreatedEvent) {
	tenantID := event.Result.TenantID()
	log.Printf("Creating chat for client %d with tenant ID: %s", event.Result.ID(), tenantID)

	// Validate tenant exists before creating chat
	ctxWithDb := composables.WithPool(context.Background(), h.pool)
	if _, err := h.tenantService.GetByID(ctxWithDb, tenantID); err != nil {
		log.Printf("failed to get tenant %s: %v", tenantID, err)
		return
	}

	ctx := h.createTenantContext(tenantID)

	if _, err := h.chatService.Save(ctx, chat.New(
		event.Result.ID(),
		chat.WithTenantID(tenantID),
	)); err != nil {
		log.Printf("failed to register client chat: %v (tenant_id: %s, client_id: %d)", err, tenantID, event.Result.ID())
		return
	}

	log.Printf("Successfully created chat for client %d", event.Result.ID())
}
