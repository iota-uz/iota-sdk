package handlers

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type SMSHandler struct {
	pool         *pgxpool.Pool
	publisher    eventbus.EventBus
	chatsService *services.ChatService
}

func RegisterSMSHandlers(
	pool *pgxpool.Pool,
	publisher eventbus.EventBus,
	chatsService *services.ChatService,
) *SMSHandler {
	handler := &SMSHandler{
		pool:         pool,
		publisher:    publisher,
		chatsService: chatsService,
	}
	publisher.Subscribe(handler.onSMSReceived)
	return handler
}

func (h *SMSHandler) onSMSReceived(event *cpassproviders.ReceivedMessageEvent) {
	ctx := context.Background()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	ctx = composables.WithPool(ctx, h.pool)
	ctx = composables.WithTx(ctx, tx)
	if err := h.chatsService.RegisterClientMessage(ctx, event); err != nil {
		log.Println(err)
		tx.Rollback(ctx)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		log.Println(err)
		return
	}
}
