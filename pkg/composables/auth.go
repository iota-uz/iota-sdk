package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var (
	ErrNoSessionFound = errors.New("no session found")
	ErrNoUserFound    = errors.New("no user found")
)

// UseUser returns the user from the context.
func UseUser(ctx context.Context) (*user.User, error) {
	u, ok := ctx.Value(constants.UserKey).(*user.User)
	if !ok {
		return nil, ErrNoUserFound
	}
	return u, nil
}

// MustUseUser returns the user from the context. If no user is found, it panics.
func MustUseUser(ctx context.Context) *user.User {
	u, err := UseUser(ctx)
	if err != nil {
		panic(err)
	}
	return u
}

func CanUser(ctx context.Context, permission *permission.Permission) error {
	u, err := UseUser(ctx)
	if err != nil {
		return err
	}
	if !u.Can(permission) {
		return nil
		// return service.ErrForbidden
	}
	return nil
}

// UseSession returns the session from the context.
func UseSession(ctx context.Context) (*session.Session, error) {
	sess, ok := ctx.Value(constants.SessionKey).(*session.Session)
	if !ok {
		return nil, ErrNoSessionFound
	}
	return sess, nil
}
