package composables

import (
	"context"
	"errors"
	"github.com/iota-agency/iota-erp/internal/domain/session"
	"github.com/iota-agency/iota-erp/internal/domain/user"
)

var (
	ErrNoSessionFound = errors.New("no session found")
	ErrNoUserFound    = errors.New("no user found")
)

// UseUser returns the user from the context.
func UseUser(ctx context.Context) (*user.User, error) {
	u, ok := ctx.Value("user").(*user.User)
	if !ok {
		return nil, ErrNoUserFound
	}
	return u, nil
}

// UseSession returns the session from the context.
func UseSession(ctx context.Context) (*session.Session, error) {
	sess, ok := ctx.Value("session").(*session.Session)
	if !ok {
		return nil, ErrNoSessionFound
	}
	return sess, nil
}
