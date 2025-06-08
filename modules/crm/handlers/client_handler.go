package handlers

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type ClientHandler struct {
	pool        *pgxpool.Pool
	publisher   eventbus.EventBus
	chatService *services.ChatService
}

func RegisterClientHandler(app application.Application) *ClientHandler {
	handler := &ClientHandler{
		pool:        app.DB(),
		publisher:   app.EventPublisher(),
		chatService: app.Service(services.ChatService{}).(*services.ChatService),
	}
	app.EventPublisher().Subscribe(handler.onCreated)
	return handler
}

func (h *ClientHandler) onCreated(event *client.CreatedEvent) {
	ctx := context.Background()
	ctx = composables.WithPool(ctx, h.pool)

	if _, err := h.chatService.Save(ctx, chat.New(
		event.Result.ID(),
		chat.WithTenantID(event.Result.TenantID()),
	)); err != nil {
		log.Printf("failed to register client chat: %v", err)
		return
	}
}
