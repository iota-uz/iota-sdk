package composables

import (
	"context"
	"errors"
	"slices"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
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

func RoleNames(u user.User) []string {
	if u == nil {
		return nil
	}
	seen := make(map[string]struct{})
	names := make([]string, 0, len(u.Roles()))
	for _, role := range u.Roles() {
		name := role.Name()
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func EffectivePermissionNames(u user.User) []string {
	if u == nil {
		return nil
	}
	seen := make(map[string]struct{})
	names := make([]string, 0, len(u.Permissions()))
	appendPermission := func(name string) {
		if name == "" {
			return
		}
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	for _, perm := range u.Permissions() {
		appendPermission(perm.Name())
	}
	for _, role := range u.Roles() {
		for _, perm := range role.Permissions() {
			appendPermission(perm.Name())
		}
	}
	slices.Sort(names)
	return names
}

// MustUseUser returns the user from the context. If no user is found, it panics.
func MustUseUser(ctx context.Context) user.User {
	u, err := UseUser(ctx)
	if err != nil {
		panic(err)
	}
	return u
}

func CanUser(ctx context.Context, perm permission.Permission) error {
	u, _ := UseUser(ctx)
	if u == nil {
		return nil
	}
	if !u.Can(perm) {
		return ErrForbidden
	}
	return nil
}

// CanUserStrict checks that a user exists in context and has the given permission.
func CanUserStrict(ctx context.Context, perm permission.Permission) error {
	const op = "composables.CanUserStrict"

	u, err := UseUser(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	if !u.Can(perm) {
		return serrors.E(op, ErrForbidden)
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
func UseSession(ctx context.Context) (session.Session, error) {
	sess, ok := ctx.Value(constants.SessionKey).(session.Session)
	if !ok {
		return nil, ErrNoSessionFound
	}
	return sess, nil
}

// WithSession returns a new context with the session.
func WithSession(ctx context.Context, sess session.Session) context.Context {
	return context.WithValue(ctx, constants.SessionKey, sess)
}
