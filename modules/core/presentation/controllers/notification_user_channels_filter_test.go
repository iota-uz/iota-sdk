package controllers_test

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/notifications/notificationstest"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationSettings_UserChannels_FilterChips covers the chip bar of
// "Channels per person": role (also through a group), group and the turned-off
// channel, applied in the directory query together with the plain ?q= search,
// kept by the save forms and carried into the next page.
func TestNotificationSettings_UserChannels_FilterChips(t *testing.T) {
	t.Parallel()
	components := modules.Components()
	for i, component := range components {
		if component.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{
				DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema(),
				NotificationChannels: []notifications.ChannelAdapter{notificationstest.NewEmail()},
			})
		}
	}
	suite := itf.NewSuiteBuilder(t).
		WithComponents(components...).
		AsUser(permissions.NotificationRulesRead, permissions.NotificationRulesManage).
		Build()
	env := suite.Env()
	tenantID := env.TenantID()
	suite.Register(controllers.NewNotificationSettingsController())
	router := itf.GetService[services.NotificationRoutingService](env)

	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	person := func(first string) uint {
		u, err := users.Create(env.Ctx, user.New(first, "Chip", internet.MustParseEmail(fmt.Sprintf("%s.%s@example.com", first, uuid.NewString()[:8])), user.UILanguageEN, user.WithTenantID(tenantID)))
		require.NoError(t, err)
		return u.ID()
	}
	anna := person("Anna")
	boris := person("Boris")
	clara := person("Clara")
	dmitry := person("Dmitry")

	var roleID uint
	require.NoError(t, env.Tx.QueryRow(env.Ctx, `INSERT INTO roles (type, name, tenant_id) VALUES ('user', 'Chip Accountant', $1) RETURNING id`, tenantID).Scan(&roleID))
	_, err := env.Tx.Exec(env.Ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, anna, roleID)
	require.NoError(t, err)
	group := uuid.New()
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO user_groups (id, type, name, tenant_id) VALUES ($1, 'user', 'Chip Sales', $2)`, group, tenantID)
	require.NoError(t, err)
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, group, boris)
	require.NoError(t, err)
	// Dmitry holds the role through a group, as a rule naming the role reaches him.
	roleGroup := uuid.New()
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO user_groups (id, type, name, tenant_id) VALUES ($1, 'user', 'Chip Accountants Group', $2)`, roleGroup, tenantID)
	require.NoError(t, err)
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2)`, roleGroup, roleID)
	require.NoError(t, err)
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, roleGroup, dmitry)
	require.NoError(t, err)
	// Clara turned email off.
	require.NoError(t, router.SetUserChannels(env.Ctx, clara, []notifications.Channel{notifications.ChannelInApp}))

	role := strconv.FormatUint(uint64(roleID), 10)
	cases := []struct {
		name    string
		query   string
		present []string
		absent  []string
	}{
		{"role", "?f=role:is:" + role, []string{"Anna Chip", "Dmitry Chip"}, []string{"Boris Chip", "Clara Chip"}},
		{"role is not", "?f=role:isnot:" + role, []string{"Boris Chip", "Clara Chip"}, []string{"Anna Chip", "Dmitry Chip"}},
		{"group", "?f=group:is:" + group.String(), []string{"Boris Chip"}, []string{"Anna Chip", "Clara Chip", "Dmitry Chip"}},
		{"turned off email", "?f=opted_out:is:email", []string{"Clara Chip"}, []string{"Anna Chip", "Boris Chip", "Dmitry Chip"}},
		{"nothing turned off", "?f=opted_out:empty:", []string{"Anna Chip", "Boris Chip", "Dmitry Chip"}, []string{"Clara Chip"}},
		{"search with chip", "?q=Anna&f=opted_out:empty:", []string{"Anna Chip"}, []string{"Boris Chip", "Clara Chip", "Dmitry Chip"}},
		// A hand-edited id matches nobody instead of failing the query.
		{"bad role id", "?f=role:is:abc", nil, []string{"Anna Chip", "Boris Chip", "Clara Chip", "Dmitry Chip"}},
	}
	for _, tc := range cases {
		response := suite.GET("/settings/notifications/users" + tc.query).HTMX().HTMXTarget("notification-user-channels-list").Expect(t).Status(200)
		response.Contains(`hx-swap-oob="outerHTML"`).Contains(`x-data="filterBuilder"`)
		for _, text := range tc.present {
			response.Contains(text)
		}
		for _, text := range tc.absent {
			response.NotContains(text)
		}
	}

	// The old link with only ?q= is still a plain search.
	suite.GET("/settings/notifications/users?q=Boris").Expect(t).Status(200).
		Contains("Boris Chip").NotContains("Anna Chip").Contains(`x-data="filterBuilder"`)

	// Opened as a full page, the search form holds the builder with the chip
	// and the search; each save form carries both back.
	full := suite.GET("/settings/notifications/users?q=Chip&f=role:is:" + role + "&fb=1").Expect(t).Status(200)
	full.Contains(`hx-trigger="submit, keyup`).Contains("filter-changed")
	body := full.Body()
	assert.Regexp(t, `name="q"[^>]*value="Chip"|value="Chip"[^>]*name="q"`, body)
	assert.Regexp(t, `type="hidden" name="f" value="role:is:`+role+`"`, body, "the save forms carry the chip")
	assert.Regexp(t, `type="hidden" name="q" value="Chip"`, body, "the save forms carry the search")

	// Saving from a filtered list re-renders that list.
	saved := suite.POST(fmt.Sprintf("/settings/notifications/users/%d", anna)).Form(url.Values{
		"channels": {"in_app"}, "q": {"Chip"}, "f": {"role:is:" + role}, "fb": {"1"},
	}).Expect(t).Status(200)
	saved.Contains(`role="status"`).Contains("Anna Chip").NotContains("Boris Chip")
	// Anna now has email off too.
	suite.GET("/settings/notifications/users?f=opted_out:is:email").HTMX().HTMXTarget("notification-user-channels-list").Expect(t).Status(200).
		Contains("Anna Chip").Contains("Clara Chip").NotContains("Boris Chip")

	// A filtered list longer than a page offers the next page with the
	// search and chips; that page is still the filtered set.
	pagers := uuid.New()
	_, err = env.Tx.Exec(env.Ctx, `INSERT INTO user_groups (id, type, name, tenant_id) VALUES ($1, 'user', 'Chip Pagers', $2)`, pagers, tenantID)
	require.NoError(t, err)
	for i := 1; i <= 21; i++ {
		id := person(fmt.Sprintf("Pager%02d", i))
		_, err = env.Tx.Exec(env.Ctx, `INSERT INTO group_users (group_id, user_id) VALUES ($1, $2)`, pagers, id)
		require.NoError(t, err)
	}
	first := suite.GET("/settings/notifications/users?q=Pager&f=group:is:" + pagers.String() + "&fb=1").Expect(t).Status(200)
	first.Contains("Pager20 Chip").NotContains("Pager21 Chip")
	match := regexp.MustCompile(`hx-get="([^"]*offset=20[^"]*)"`).FindStringSubmatch(first.Body())
	require.NotNil(t, match, "the first page offers the next one")
	link, err := url.Parse(html.UnescapeString(match[1]))
	require.NoError(t, err)
	got := link.Query()
	assert.Equal(t, []string{"group:is:" + pagers.String()}, got["f"])
	assert.Equal(t, "1", got.Get("fb"))
	assert.Equal(t, "Pager", got.Get("q"))
	next := suite.GET(link.Path + "?" + link.RawQuery).HTMX().HTMXTarget("user-channels-more").Expect(t).Status(200)
	next.Contains("Pager21 Chip").NotContains("Pager01 Chip").NotContains("Anna Chip").NotContains("offset=")
	// Rows of the second page save back to their own page, so the saved row
	// stays on screen.
	next.Contains(`name="offset" value="20"`)
}
