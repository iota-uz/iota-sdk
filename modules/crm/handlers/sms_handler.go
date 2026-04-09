// Package handlers provides this package.
package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"

	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
)

// SMSHandler subscribes to inbound SMS events. Register with
// composition.ProvideFunc + ContributeHooks — lifecycle is managed by the
// engine, not by the handler.
type SMSHandler struct {
	pool        *pgxpool.Pool
	chatService *services.ChatService
}

func NewSMSHandler(pool *pgxpool.Pool, chatService *services.ChatService) *SMSHandler {
	return &SMSHandler{
		pool:        pool,
		chatService: chatService,
	}
}

// OnSMSReceived is the eventbus subscriber callback. Compatible with
// eventbus.EventBus.Subscribe.
func (h *SMSHandler) OnSMSReceived(event *cpassproviders.ReceivedMessageEvent) {
	// ctx := context.Background()
	// ctx = composables.WithPool(ctx, h.pool)
	//
	//	if _, err := h.chatService.RegisterClientMessage(ctx, event); err != nil {
	//		log.Printf("failed to register client message: %v", err)
	//		return
	//	}
}
