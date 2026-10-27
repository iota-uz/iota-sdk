package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

type navApp struct {
	application.Application
	items []types.NavigationItem
}

func (a navApp) NavItems(*i18n.Localizer) []types.NavigationItem { return a.items }

func (a navApp) NavWorkspaces() []types.NavWorkspace { return nil }

// A user without own roles still holds permissions granted directly or through
// a group role; the sidebar must follow the same permissions as the routes.
func TestNavItemsWithInitialState_FollowsEffectivePermissions(t *testing.T) {
	t.Parallel()
	app := navApp{items: []types.NavigationItem{
		{Key: "portfolio", Href: "/portfolio/policies", Permissions: []permission.Permission{permA}},
		{Key: "reinsurance", Href: "/reinsurance/contracts", Permissions: []permission.Permission{permB}},
		{Key: "help", Href: "/help"},
	}}
	localizer := i18n.NewLocalizer(i18n.NewBundle(language.English), language.English.String())
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
			ctx := context.WithValue(context.Background(), constants.AppKey, application.Application(app))
			ctx = intl.WithLocalizer(ctx, localizer)
			ctx = composables.WithUser(ctx, tt.user)
			req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)

			var got []types.NavigationItem
			handler := NavItemsWithInitialState(sidebar.SidebarAuto)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got, _ = r.Context().Value(constants.NavItemsKey).([]types.NavigationItem)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			require.Equal(t, tt.want, keysOf(got))
		})
	}
}
