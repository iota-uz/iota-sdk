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
// If the user is not found, the second return value will be false.
func UseUser(ctx context.Context) (*user.User, bool) {
	u, ok := ctx.Value("user").(*user.User)
	if !ok {
		return nil, false
	}
	return u, true
}

// UseSession returns the session from the context.
// If the session is not found, the second return value will be false.
func UseSession(ctx context.Context) (*session.Session, bool) {
	sess, ok := ctx.Value("session").(*session.Session)
	if !ok {
		return nil, false
	}
	return sess, true
}
