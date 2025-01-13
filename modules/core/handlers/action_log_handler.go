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

func (h *ActionLogEventHandler) onSessionCreated(event user.CreatedEvent) {
	sess := event.Result
	//logEntity := &authlog.AuthenticationLog{
	//	ID:        0,
	//	UserID:    sess.UserID,
	//	IP:        sess.IP,
	//	UserAgent: sess.UserAgent,
	//	CreatedAt: time.Now(),
	//}
	//tx, err := h.pool.Begin(context.Background())
	//if err != nil {
	//	log.Fatalf("failed to begin transaction: %v", err)
	//}
	//ctx := composables.WithTx(context.Background(), tx)
	//if err := h.authLogService.Create(ctx, logEntity); err != nil {
	//	log.Fatalf("failed to create auth log: %v", err)
	//}
	//if err := tx.Commit(context.Background()); err != nil {
	//	log.Fatalf("failed to commit transaction: %v", err)
	//}
}
