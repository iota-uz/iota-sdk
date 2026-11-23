package controllers_test

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionRows returns the list part of a sessions response, without the chip
// bar: user names also appear as options of the "user" chip, so only the rows
// tell which sessions were listed. The full page has the bar above the list,
// an HTMX answer has it below, out-of-band.
func sessionRows(t *testing.T, body string) string {
	t.Helper()
	if i := strings.Index(body, `id="sessions-list"`); i >= 0 {
		body = body[i:]
	}
	rows, _, _ := strings.Cut(body, `id="session-filters"`)
	return rows
}

// TestSessionController_List_FilterChips covers the chip bar of the admin
// sessions list: user, sign-in state and dates, together with the plain
// ?search=, the out-of-band chip bar on HTMX responses and the conditions
// carried into the next page of infinite scroll.
func TestSessionController_List_FilterChips(t *testing.T) {
	t.Parallel()
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.SessionRead, permissions.UserRead).
		Build()
	env := suite.Env()
	persistTestUser(t, env)
	suite.Register(controllers.NewSessionController("/settings/sessions", itf.GetService[cookies.Config](env)))
	sessionService := itf.GetService[services.SessionService](env)

	addUser := func(offset uint, first, email string) uint {
		id := env.User.ID() + offset
		_, err := env.Pool.Exec(env.Ctx, `
			INSERT INTO users (id, type, first_name, last_name, email, password, tenant_id, ui_language, created_at, updated_at)
			VALUES ($1, 'user', $2, 'Chip', $3, '', $4, 'en', NOW(), NOW())`, id, first, email, env.Tenant.ID)
		require.NoError(t, err)
		return id
	}
	alice := addUser(20_001, "Alice", "alice.chip@example.com")
	bob := addUser(20_002, "Bob", "bob.chip@example.com")

	addSession := func(token string, userID uint, ip string, status session.SessionStatus, createdAt string) {
		require.NoError(t, sessionService.Create(env.Ctx, &session.CreateDTO{
			UserID: userID, TenantID: env.Tenant.ID, IP: ip, UserAgent: "chip-agent", Token: token, Status: status,
		}))
		_, err := env.Tx.Exec(env.Ctx, `UPDATE sessions SET created_at = $1::timestamptz WHERE token = $2`, createdAt, token)
		require.NoError(t, err)
	}
	addSession("chip-alice-old", alice, "10.1.0.1", session.StatusActive, "2026-01-10T10:00:00Z")
	addSession("chip-alice-2fa", alice, "10.1.0.2", session.StatusPending2FA, "2026-03-10T10:00:00Z")
	addSession("chip-bob", bob, "10.2.0.1", session.StatusActive, "2026-03-12T10:00:00Z")

	aliceID := strconv.FormatUint(uint64(alice), 10)
	cases := []struct {
		name    string
		query   string
		present []string
		absent  []string
	}{
		{"user", "?f=user:is:" + aliceID, []string{"10.1.0.1", "10.1.0.2"}, []string{"10.2.0.1"}},
		{"user is not", "?f=user:isnot:" + aliceID, []string{"10.2.0.1"}, []string{"10.1.0.1", "10.1.0.2"}},
		{"status", "?f=status:is:pending_2fa", []string{"10.1.0.2"}, []string{"10.1.0.1", "10.2.0.1"}},
		{"created after", "?f=created_at:after:2026-02-01", []string{"10.1.0.2", "10.2.0.1"}, []string{"10.1.0.1"}},
		{"user and status", "?f=user:is:" + aliceID + "&f=status:is:active", []string{"10.1.0.1"}, []string{"10.1.0.2", "10.2.0.1"}},
		{"search with chip", "?search=Bob&f=status:is:active", []string{"10.2.0.1"}, []string{"10.1.0.1", "10.1.0.2"}},
		// A hand-edited id matches nothing instead of failing the query.
		{"bad user id", "?f=user:is:abc", nil, []string{"10.1.0.1", "10.1.0.2", "10.2.0.1"}},
	}
	for _, tc := range cases {
		response := suite.GET("/settings/sessions" + tc.query).HTMX().HTMXTarget("sessions-list").Expect(t).Status(200)
		body := response.Body()
		require.Contains(t, body, `id="session-filters"`, tc.name)
		require.Contains(t, body, `hx-swap-oob="outerHTML"`, tc.name)
		rows := sessionRows(t, body)
		for _, text := range tc.present {
			require.Contains(t, rows, text, tc.name)
		}
		for _, text := range tc.absent {
			require.NotContains(t, rows, text, tc.name)
		}
	}

	// The old link with only ?search= still filters, without any chip.
	legacy := suite.GET("/settings/sessions?search=Alice").Expect(t).Status(200)
	legacyRows := sessionRows(t, legacy.Body())
	require.Contains(t, legacyRows, "10.1.0.1")
	require.NotContains(t, legacyRows, "10.2.0.1")
	// The old form target (rows only) still gets rows, plus the chip bar.
	oldTarget := suite.GET("/settings/sessions?search=Bob").HTMX().HTMXTarget("sessions-table-body").Expect(t).Status(200)
	require.Contains(t, sessionRows(t, oldTarget.Body()), "10.2.0.1")
	require.NotContains(t, sessionRows(t, oldTarget.Body()), "10.1.0.1")
	oldTarget.Contains(`hx-swap-oob="outerHTML"`)

	// The full page renders the builder inside the search form, with the chip
	// and the search kept for the next change.
	full := suite.GET("/settings/sessions?search=Alice&f=status:is:active&fb=1").Expect(t).Status(200)
	full.Contains(`x-data="filterBuilder"`).Contains(`value="status:is:active"`).Contains("filter-changed")
	assert.Regexp(t, `name="search"[^>]*value="Alice"|value="Alice"[^>]*name="search"`, full.Body())
	require.Contains(t, sessionRows(t, full.Body()), "10.1.0.1")
	require.NotContains(t, sessionRows(t, full.Body()), "10.1.0.2")

	// Infinite scroll: with one session per page, the last row asks for page
	// 2 with the search and chips; that page is still Alice's.
	first := suite.GET("/settings/sessions?search=Alice&f=user:is:" + aliceID + "&fb=1&limit=1").Expect(t).Status(200)
	match := regexp.MustCompile(`hx-get="([^"]*[?&](?:amp;)?page=2[^"]*)"`).FindStringSubmatch(first.Body())
	require.NotNil(t, match, "the last row asks for page 2")
	link, err := url.Parse(html.UnescapeString(match[1]))
	require.NoError(t, err)
	got := link.Query()
	assert.Equal(t, []string{"user:is:" + aliceID}, got["f"])
	assert.Equal(t, "1", got.Get("fb"))
	assert.Equal(t, "Alice", got.Get("search"))
	assert.Equal(t, "1", got.Get("limit"))
	second := suite.GET(link.Path + "?" + link.RawQuery).HTMX().Expect(t).Status(200)
	body := second.Body()
	assert.True(t, strings.Contains(body, "10.1.0.1") || strings.Contains(body, "10.1.0.2"), "page 2 shows one of Alice's sessions")
	second.NotContains("10.2.0.1")
	// Alice has two sessions, so page 2 of one is the last.
	second.NotContains(`page=3`)
}

// TestSessionController_List_FilteredCount checks that "has more" follows the
// filtered total, not the whole tenant: Alice's single session must not ask
// for a second page although the tenant has several.
func TestSessionController_List_FilteredCount(t *testing.T) {
	t.Parallel()
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.SessionRead, permissions.UserRead).
		Build()
	env := suite.Env()
	persistTestUser(t, env)
	suite.Register(controllers.NewSessionController("/settings/sessions", itf.GetService[cookies.Config](env)))
	sessionService := itf.GetService[services.SessionService](env)
	for i := 1; i <= 3; i++ {
		require.NoError(t, sessionService.Create(env.Ctx, &session.CreateDTO{
			UserID: env.User.ID(), TenantID: env.Tenant.ID, IP: fmt.Sprintf("10.9.0.%d", i), UserAgent: "count-agent",
			Token: fmt.Sprintf("count-token-%d", i), Status: session.StatusActive,
		}))
	}
	require.NoError(t, sessionService.Create(env.Ctx, &session.CreateDTO{
		UserID: env.User.ID(), TenantID: env.Tenant.ID, IP: "10.9.1.1", UserAgent: "count-agent",
		Token: "count-token-2fa", Status: session.StatusPending2FA,
	}))
	only := suite.GET("/settings/sessions?f=status:is:pending_2fa&limit=1").Expect(t).Status(200)
	only.Contains("10.9.1.1").NotContains("page=2")
	all := suite.GET("/settings/sessions?limit=1").Expect(t).Status(200)
	all.Contains("page=2")
}
