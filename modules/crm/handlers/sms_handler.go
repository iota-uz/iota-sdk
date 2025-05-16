package handlers

import (
	"github.com/jackc/pgx/v5/pgxpool"

	cpassproviders "github.com/iota-uz/iota-sdk/modules/crm/infrastructure/cpass-providers"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
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
	// ctx := context.Background()
	// ctx = composables.WithPool(ctx, h.pool)
	//
	//	if _, err := h.chatService.RegisterClientMessage(ctx, event); err != nil {
	//		log.Printf("failed to register client message: %v", err)
	//		return
	//	}
}
