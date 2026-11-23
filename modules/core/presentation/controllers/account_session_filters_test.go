package controllers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// TestAccountSessionController_FilterChips covers the chip bar of the user's
// own sessions: sign-in date, expiry date and "current session" narrow the
// list, and htmx answers carry the builder out-of-band.
func TestAccountSessionController_FilterChips(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser().Build()
	persistTestUser(t, suite.Env())
	suite.Register(newAccountController(suite))
	env := suite.Env()
	cfg := itf.GetService[cookies.Config](env)
	sessionService := itf.GetService[services.SessionService](env)

	const currentToken = "chips-current-token"
	for _, s := range []struct {
		token string
		ip    string
	}{{currentToken, "127.0.0.1"}, {"chips-old-token", "10.0.0.1"}, {"chips-recent-token", "10.0.0.2"}} {
		require.NoError(t, sessionService.Create(env.Ctx, &session.CreateDTO{
			UserID: env.User.ID(), TenantID: env.Tenant.ID, IP: s.ip, UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", Token: s.token,
		}))
	}
	// Sessions are written in the suite transaction; date them there too.
	tx, err := composables.UseTx(env.Ctx)
	require.NoError(t, err)
	tag, err := tx.Exec(env.Ctx, `UPDATE sessions SET created_at = $1 WHERE token = $2`, time.Date(2020, 3, 10, 12, 0, 0, 0, time.UTC), "chips-old-token")
	require.NoError(t, err)
	require.EqualValues(t, 1, tag.RowsAffected())

	row := func(token string) string { return fmt.Sprintf(`id="session-row-%s"`, hashToken(token)) }
	get := func(query string) *itf.Response {
		return suite.GET("/account/sessions"+query).Cookie(cfg.SID, currentToken).Expect(t).Status(http.StatusOK)
	}

	t.Run("Without chips every session is listed with the builder", func(t *testing.T) {
		body := get("").Body()
		require.Contains(t, body, `x-data="filterBuilder"`)
		for _, token := range []string{currentToken, "chips-old-token", "chips-recent-token"} {
			require.Contains(t, body, row(token))
		}
	})

	t.Run("Sign-in date chip", func(t *testing.T) {
		body := get("?fb=1&f=created_at:before:2021-01-01").Body()
		require.Contains(t, body, `value="created_at:before:2021-01-01"`)
		require.Contains(t, body, row("chips-old-token"))
		require.NotContains(t, body, row("chips-recent-token"))
		require.NotContains(t, body, row(currentToken))
		// Revoking all other sessions stays available although the chips hide them.
		require.Contains(t, body, `hx-delete="/account/sessions/others"`)
	})

	t.Run("Expiry date chip", func(t *testing.T) {
		body := get("?fb=1&f=expires_at:before:2021-01-01").Body()
		require.NotContains(t, body, row(currentToken))
		require.Contains(t, body, "No sessions match these filters")
		body = get("?fb=1&f=expires_at:after:2021-01-01").Body()
		require.Contains(t, body, row(currentToken))
	})

	t.Run("Current session chip", func(t *testing.T) {
		body := get("?fb=1&f=current:is:true").Body()
		require.Contains(t, body, row(currentToken))
		require.NotContains(t, body, row("chips-old-token"))
		body = get("?fb=1&f=current:is:false").Body()
		require.NotContains(t, body, row(currentToken))
		require.Contains(t, body, row("chips-old-token"))
		require.Contains(t, body, row("chips-recent-token"))
	})

	t.Run("Htmx answer re-renders the list with the builder out-of-band", func(t *testing.T) {
		body := suite.GET("/account/sessions?fb=1&f=current:is:false").Cookie(cfg.SID, currentToken).HTMX().
			Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, `id="account-sessions-results"`)
		require.Contains(t, body, `hx-swap-oob="outerHTML"`)
		require.Contains(t, body, `id="account-session-filters"`)
		require.Contains(t, body, row("chips-old-token"))
		require.NotContains(t, body, row(currentToken))
		require.NotContains(t, body, "<html")
	})

	t.Run("history restore gets the whole page", func(t *testing.T) {
		body := suite.GET("/account/sessions?f=current:is:false&fb=1").Cookie(cfg.SID, currentToken).HTMX().HTMXHistoryRestore().Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, "<html")
		require.Contains(t, body, `x-data="filterBuilder"`)
	})
}
