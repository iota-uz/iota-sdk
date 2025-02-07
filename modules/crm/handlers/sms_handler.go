package handlers

import (
	"context"
	"log"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"

	"github.com/jackc/pgx/v5/pgxpool"

	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SMSHandler struct {
	pool        *pgxpool.Pool
	publisher   eventbus.EventBus
	chatService *services.ChatService
}

func RegisterSMSHandlers(app application.Application) *SMSHandler {
	handler := &SMSHandler{
		pool:        app.DB(),
		publisher:   app.EventPublisher(),
		chatService: app.Service(services.ChatService{}).(*services.ChatService),
	}
	app.EventPublisher().Subscribe(handler.onSMSReceived)
	return handler
}

func (h *SMSHandler) onSMSReceived(event *cpassproviders.ReceivedMessageEvent) {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Printf("failed to start transaction: %v", err)
		return
	}
	ctx = composables.WithPool(ctx, h.pool)
	ctx = composables.WithTx(ctx, tx)
	chatEntity, err := h.chatService.RegisterClientMessage(ctx, event)
	if err != nil {
		log.Printf("failed to register client message: %v", err)
		tx.Rollback(ctx)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return
	}

	ev, err := chat.NewMessageAddedEvent(ctx, chatEntity)
	if err != nil {
		log.Printf("failed to create created message event: %v", err)
	}

	h.publisher.Publish(ev)
}
