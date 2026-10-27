package middleware

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

// A user without own roles still holds permissions granted directly or through
// a group role; the navigation must follow those permissions.
func TestVisibleNavItems_FollowsEffectivePermissions(t *testing.T) {
	t.Parallel()
	items := []types.NavigationItem{
		{Key: "portfolio", Href: "/portfolio/policies", Permissions: []permission.Permission{permA}},
		{Key: "reinsurance", Href: "/reinsurance/contracts", Permissions: []permission.Permission{permB}},
		{Key: "help", Href: "/help"},
	}
	newUser := func(opts ...user.Option) user.User {
		return user.New("Test", "User", internet.MustParseEmail("test@example.com"), user.UILanguageEN, opts...)
	}

	tests := []struct {
		name string
		user user.User
		want []string
	}{
		{name: "no permissions", user: newUser(), want: []string{}},
		{name: "own role", user: newUser(user.WithRoles([]role.Role{role.New("Portfolio", role.WithPermissions([]permission.Permission{permA}))})), want: []string{"portfolio", "help"}},
		{name: "direct permission without roles", user: newUser(user.WithPermissions([]permission.Permission{permA})), want: []string{"portfolio", "help"}},
		{name: "group role without own roles", user: newUser(user.WithGroupPermissions([]permission.Permission{permB})), want: []string{"reinsurance", "help"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, keysOf(visibleNavItems(items, tt.user)))
		})
	}
}
