package handlers

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SMSHandler struct {
	pool            *pgxpool.Pool
	publisher       eventbus.EventBus
	messagesService *services.MessagesService
}

func RegisterSMSHandlers(
	pool *pgxpool.Pool,
	publisher eventbus.EventBus,
	messagesService *services.MessagesService,
) *SMSHandler {
	handler := &SMSHandler{
		pool:            pool,
		publisher:       publisher,
		messagesService: messagesService,
	}
	publisher.Subscribe(handler.onSMSReceived)
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
	createdMessage, err := h.messagesService.RegisterClientMessage(ctx, event)
	if err != nil {
		log.Printf("failed to register client message: %v", err)
		tx.Rollback(ctx)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("failed to commit transaction: %v", err)
		return
	}

	ev, err := message.NewCreatedEvent(ctx, createdMessage)
	if err != nil {
		log.Printf("failed to create created message event: %v", err)
	}

	h.publisher.Publish(ev)
}
