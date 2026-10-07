//go:build dev

package services

import (
	"context"
	"errors"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/agentsession"
)

// CreateAgentSession bypasses login gates only in a development build and environment.
func (s *AuthService) CreateAgentSession(ctx context.Context, u user.User) (session.Session, error) {
	if !agentsession.Allowed(s.appCfg.Environment) {
		return nil, errors.New("agent sessions require local development")
	}
	return s.createSession(ctx, u, agentsession.Audience, session.StatusActive, time.Now().Add(time.Hour))
}
