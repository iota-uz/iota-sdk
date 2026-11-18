package authorizers

import (
	"context"
	"slices"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// SessionDirectiveAuthorizer backs the @authenticated and @can GraphQL
// directives with the user that middleware.ProvideUser put into the context.
// @can permissions are matched against the user's effective permission names
// (direct, role and group grants). It satisfies pkg/graphql.Authorizer; the
// package does not import pkg/graphql to keep its tests free of import cycles.
type SessionDirectiveAuthorizer struct{}

// NewSessionDirectiveAuthorizer returns the default directive authorizer.
func NewSessionDirectiveAuthorizer() SessionDirectiveAuthorizer {
	return SessionDirectiveAuthorizer{}
}

// Authenticated requires a user in the context.
func (SessionDirectiveAuthorizer) Authenticated(ctx context.Context) error {
	const op serrors.Op = "SessionDirectiveAuthorizer.Authenticated"

	if _, err := composables.UseUser(ctx); err != nil {
		return serrors.Wrap(op, serrors.NewUnauthenticated("authentication required"))
	}
	return nil
}

// Can requires a user in the context that holds every permission.
func (a SessionDirectiveAuthorizer) Can(ctx context.Context, permissions []string) error {
	const op serrors.Op = "SessionDirectiveAuthorizer.Can"

	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.Wrap(op, serrors.NewUnauthenticated("authentication required"))
	}
	granted := composables.EffectivePermissionNames(u)
	for _, perm := range permissions {
		if _, found := slices.BinarySearch(granted, perm); !found {
			return serrors.Wrap(op, serrors.NewPermissionDenied("permission denied"))
		}
	}
	return nil
}
