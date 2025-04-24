package handlers

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
)

type UserHandler struct {
	pool           *pgxpool.Pool
	publisher      eventbus.EventBus
	sessionService *services.SessionService
}

func RegisterUserHandler(app application.Application) *UserHandler {
	handler := &UserHandler{
		pool:           app.DB(),
		publisher:      app.EventPublisher(),
		sessionService: app.Service(services.SessionService{}).(*services.SessionService),
	}
	app.EventPublisher().Subscribe(handler.onUserPasswordUpdated)
	return handler
}

func (h *UserHandler) onUserPasswordUpdated(event *user.UpdatedPasswordEvent) {
	ctx := context.Background()
	ctx = composables.WithPool(ctx, h.pool)

	if _, err := h.sessionService.DeleteByUserId(ctx, event.UserID); err != nil {
		log.Printf("failed to register client chat: %v", err)
		return
	}
}
