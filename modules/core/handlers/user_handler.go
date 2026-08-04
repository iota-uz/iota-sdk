// Package handlers provides this package.
package handlers

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/services"

	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// UserHandler reacts to user-level domain events. Its sole responsibility
// today is revoking active sessions after a password change so that the
// previous credentials stop being honoured system-wide.
type UserHandler struct {
	pool           *pgxpool.Pool
	sessionService *services.SessionService
	logger         *logrus.Logger
}

// NewUserHandler is the reflection-injector-friendly constructor wired from
// the core component via composition.ProvideFunc.
func NewUserHandler(
	pool *pgxpool.Pool,
	sessionService *services.SessionService,
	logger *logrus.Logger,
) *UserHandler {
	return &UserHandler{
		pool:           pool,
		sessionService: sessionService,
		logger:         logger,
	}
}

// OnPasswordUpdated revokes all sessions owned by the user whose password
// changed. Failures are logged but not returned: the event bus cannot
// recover from a handler error and callers would be blocked on a non-fatal
// session cleanup problem.
func (h *UserHandler) OnPasswordUpdated(event *user.UpdatedPasswordEvent) {
	ctx := composables.WithPool(context.Background(), h.pool)
	if _, err := h.sessionService.DeleteByUserID(ctx, event.UserID); err != nil && h.logger != nil {
		h.logger.WithError(err).WithField("user_id", event.UserID).
			Warn("failed to revoke sessions after password change")
	}
}
