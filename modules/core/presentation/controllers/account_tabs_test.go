package controllers_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/account"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

// TestAccountController_Tabs covers the tabbed account page: every tab has an
// address that serves the whole page, a tab switch (htmx aimed at the tab
// panel) gets the tab alone, and the ?tab= links of the old page still land.
func TestAccountController_Tabs(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser().Build()
	persistTestUser(t, suite.Env())
	suite.Register(newAccountController(suite))
	cfg := itf.GetService[cookies.Config](suite.Env())
	const token = "account-tabs-token"
	createTestSession(t, suite.Env(), token)

	tabs := []struct {
		path   string
		marker string
	}{
		{"/account", `data-testid="account-profile-form"`},
		{"/account/password", `data-testid="change-password-form"`},
		{"/account/sessions", `data-testid="account-sessions"`},
	}

	for _, tab := range tabs {
		t.Run("full page "+tab.path, func(t *testing.T) {
			body := suite.GET(tab.path).Cookie(cfg.SID, token).Expect(t).Status(http.StatusOK).Body()
			require.Contains(t, body, "<html")
			require.Contains(t, body, `id="`+account.TabPanelID+`"`)
			require.Contains(t, body, tab.marker)
			for _, other := range account.Tabs {
				require.Contains(t, body, `href="`+other.Path+`"`)
			}
		})

		t.Run("tab switch "+tab.path, func(t *testing.T) {
			body := suite.GET(tab.path).Cookie(cfg.SID, token).HTMX().HTMXTarget(account.TabPanelID).
				Expect(t).Status(http.StatusOK).Body()
			require.NotContains(t, body, "<html")
			require.NotContains(t, body, `id="`+account.TabPanelID+`"`)
			require.Contains(t, body, "<title>")
			require.Contains(t, body, tab.marker)
		})

		t.Run("history restore "+tab.path, func(t *testing.T) {
			body := suite.GET(tab.path).Cookie(cfg.SID, token).HTMX().HTMXTarget(account.TabPanelID).HTMXHistoryRestore().
				Expect(t).Status(http.StatusOK).Body()
			require.Contains(t, body, "<html")
		})
	}

	t.Run("the old ?tab= link goes to the tab", func(t *testing.T) {
		suite.GET("/account?tab=password").Cookie(cfg.SID, token).Expect(t).Status(http.StatusFound).RedirectTo("/account/password")
	})

	t.Run("a chip change of the sessions tab still gets the list only", func(t *testing.T) {
		body := suite.GET("/account/sessions").Cookie(cfg.SID, token).HTMX().HTMXTarget("account-sessions-results").
			Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, `id="account-sessions-results"`)
		require.NotContains(t, body, `data-testid="account-sessions"`)
	})
}

// TestAccountController_ChangePasswordErrors covers the inline errors of the
// password tab: the form comes back with the field's message, never a page.
func TestAccountController_ChangePasswordErrors(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser().Build()
	persistTestUser(t, suite.Env())
	suite.Register(newAccountController(suite))
	cfg := itf.GetService[cookies.Config](suite.Env())
	const token = "account-password-token"
	createTestSession(t, suite.Env(), token)

	post := func(current, next, confirm string) string {
		return suite.POST(account.PasswordPath).Cookie(cfg.SID, token).HTMX().Form(url.Values{
			"CurrentPassword": {current}, "NewPassword": {next}, "ConfirmPassword": {confirm},
		}).Expect(t).Status(http.StatusOK).Body()
	}

	t.Run("short and mismatched", func(t *testing.T) {
		body := post("", "short", "other")
		require.Contains(t, body, `data-testid="change-password-form"`)
		require.NotContains(t, body, "<html")
		require.Contains(t, body, "Enter your current password.")
		require.Contains(t, body, "The new password must contain at least 8 characters.")
		require.Contains(t, body, "The password confirmation does not match.")
	})

	t.Run("wrong current password", func(t *testing.T) {
		body := post("NotThePassword1!", "ChangedPass123!", "ChangedPass123!")
		require.Contains(t, body, "The current password is incorrect.")
	})
}

func createTestSession(t *testing.T, env *itf.TestEnvironment, token string) {
	t.Helper()
	sessionService := itf.GetService[services.SessionService](env)
	require.NoError(t, sessionService.Create(env.Ctx, &session.CreateDTO{
		UserID: env.User.ID(), TenantID: env.Tenant.ID, IP: "127.0.0.1", UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)", Token: token,
	}))
}
