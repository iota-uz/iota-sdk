package composables

import (
	"context"
	"errors"

	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/session"
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

func CanUser(ctx context.Context, permission permission.Permission) error {
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
