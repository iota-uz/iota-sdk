package handlers

import (
	"context"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/authlog"
	"github.com/iota-agency/iota-erp/internal/domain/entities/session"
	"github.com/iota-agency/iota-erp/sdk/composables"
	"github.com/iota-agency/iota-erp/sdk/event"
	"gorm.io/gorm"
	"log"
	"time"
)

type SessionEventsHandler struct {
	db             *gorm.DB
	publisher      event.Publisher
	authLogService *services.AuthLogService
}

func NewSessionEventsHandler(
	db *gorm.DB,
	publisher event.Publisher,
	authLogService *services.AuthLogService,
) *SessionEventsHandler {
	handler := &SessionEventsHandler{
		db:             db,
		publisher:      publisher,
		authLogService: authLogService,
	}
	publisher.Subscribe(handler.OnSessionCreated)
	return handler
}

func (h *SessionEventsHandler) OnSessionCreated(event session.CreatedEvent) {
	sess := event.Session
	logEntity := &authlog.AuthenticationLog{
		UserID:    sess.UserID,
		IP:        sess.IP,
		UserAgent: sess.UserAgent,
		CreatedAt: time.Now(),
	}
	tx := h.db.Begin()
	ctx := composables.WithTx(context.Background(), tx)
	if err := h.authLogService.Create(ctx, logEntity); err != nil {
		log.Fatalf("failed to create auth log: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		log.Fatalf("failed to commit transaction: %v", err)
	}
}
