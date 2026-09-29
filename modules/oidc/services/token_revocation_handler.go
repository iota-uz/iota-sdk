package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

// TokenRevocationHandler revokes a user's refresh tokens when the password is
// replaced, so a reset also ends access granted to OIDC clients.
type TokenRevocationHandler struct {
	pool      *pgxpool.Pool
	tokenRepo token.Repository
	logger    *logrus.Logger
}

func NewTokenRevocationHandler(pool *pgxpool.Pool, tokenRepo token.Repository, logger *logrus.Logger) *TokenRevocationHandler {
	return &TokenRevocationHandler{pool: pool, tokenRepo: tokenRepo, logger: logger}
}

func (h *TokenRevocationHandler) OnPasswordUpdated(event *user.UpdatedPasswordEvent) {
	ctx := composables.WithPool(context.Background(), h.pool)
	if err := h.tokenRepo.DeleteByUserID(ctx, int(event.UserID)); err != nil && h.logger != nil {
		h.logger.WithError(err).WithField("user_id", event.UserID).
			Warn("failed to revoke refresh tokens after password change")
	}
}
