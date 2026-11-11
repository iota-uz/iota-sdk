package native

import (
	"context"
	"slices"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ResolveDispatcher returns the application's dispatcher: the one a component
// provides (the extension runtime provides its host dispatcher), otherwise a
// Direct dispatcher over the Go defaults of Default bound to the container,
// authorizing with the request user's permissions. Modules that publish native
// commands resolve it once and route every entry of those commands through it.
func ResolveDispatcher(c *composition.Container) (Dispatcher, error) {
	d, err := composition.Resolve[Dispatcher](c)
	if err == nil {
		return d, nil
	}
	if !composition.IsNotProvided(err) {
		return nil, serrors.Wrap("extruntime.native.ResolveDispatcher", err)
	}
	return NewDirect(Bind(Default, c), WithAuthorizer(UserAuthorizer)), nil
}

// UserAuthorizer checks a permission against the effective permissions of the
// request user (composables.UseUser). A context without a user is refused.
func UserAuthorizer(ctx context.Context, name string) bool {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(user.EffectivePermissions(u), func(p permission.Permission) bool { return p.Name() == name })
}
