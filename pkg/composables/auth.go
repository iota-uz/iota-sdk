package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
)

var (
	ErrNoSessionFound = errors.New("no session found")
	ErrNoUserFound    = errors.New("no user found")
)

// UseUser returns the user from the context.
func UseUser(ctx context.Context) (user.User, error) {
	u, ok := ctx.Value(constants.UserKey).(user.User)
	if !ok {
		return nil, ErrNoUserFound
	}
	return u, nil
}

// WithUser returns a new context with the user.
func WithUser(ctx context.Context, u user.User) context.Context {
	return context.WithValue(ctx, constants.UserKey, u)
}

// MustUseUser returns the user from the context. If no user is found, it panics.
func MustUseUser(ctx context.Context) user.User {
	u, err := UseUser(ctx)
	if err != nil {
		panic(err)
	}
	return u
}

func CanUser(ctx context.Context, permission *permission.Permission) error {
	u, _ := UseUser(ctx)
	if u == nil {
		return nil
	}
	if !u.Can(permission) {
		return ErrForbidden
	}
	return nil
}

func CanUserAll(ctx context.Context, perms ...rbac.Permission) error {
	u, _ := UseUser(ctx)
	if u == nil || len(perms) == 0 {
		return nil // don't check if the user isn't in the context
	}
	if !rbac.And(perms...).Can(u) {
		return ErrForbidden
	}
	return nil
}

// CanUserAny checks if the user has any of the given permissions (OR logic)
func CanUserAny(ctx context.Context, perms ...rbac.Permission) error {
	u, _ := UseUser(ctx)
	if u == nil || len(perms) == 0 {
		return nil // don't check if the user isn't in the context
	}
	if !rbac.Or(perms...).Can(u) {
		return ErrForbidden
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

// WithSession returns a new context with the session.
func WithSession(ctx context.Context, sess *session.Session) context.Context {
	return context.WithValue(ctx, constants.SessionKey, sess)
}
