// Package handlers provides this package.
package handlers

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ActionLogEventHandler struct {
	pool           *pgxpool.Pool
	publisher      eventbus.EventBus
	authLogService *services.AuthLogService
}

func RegisterActionLogEventHandlers(
	pool *pgxpool.Pool,
	eventBus eventbus.EventBus,
	authLogService *services.AuthLogService,
) *ActionLogEventHandler {
	handler := &ActionLogEventHandler{
		pool:           pool,
		publisher:      eventBus,
		authLogService: authLogService,
	}
	eventBus.Subscribe(handler.onSessionCreated)
	return handler
}

func (h *ActionLogEventHandler) onSessionCreated(_ user.CreatedEvent) {
}
