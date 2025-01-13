package handlers

import (
	"context"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"time"
)

type SessionEventsHandler struct {
	pool           *pgxpool.Pool
	publisher      eventbus.EventBus
	authLogService *services.AuthLogService
}

func RegisterSessionEventHandlers(
	pool *pgxpool.Pool,
	publisher eventbus.EventBus,
	authLogService *services.AuthLogService,
) *SessionEventsHandler {
	handler := &SessionEventsHandler{
		pool:           pool,
		publisher:      publisher,
		authLogService: authLogService,
	}
	publisher.Subscribe(handler.onSessionCreated)
	return handler
}

func (h *SessionEventsHandler) onSessionCreated(event session.CreatedEvent) {
	sess := event.Result
	logEntity := &authlog.AuthenticationLog{
		ID:        0,
		UserID:    sess.UserID,
		IP:        sess.IP,
		UserAgent: sess.UserAgent,
		CreatedAt: time.Now(),
	}
	tx, err := h.pool.Begin(context.Background())
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}
	ctx := composables.WithTx(context.Background(), tx)
	if err := h.authLogService.Create(ctx, logEntity); err != nil {
		log.Fatalf("failed to create auth log: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		log.Fatalf("failed to commit transaction: %v", err)
	}
}
