package controllers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestLogoutController_Scenarios(t *testing.T) {
	t.Parallel()

	type cfgBundle struct {
		httpCfg    *httpconfig.Config
		cookiesCfg *cookies.Config
		appCfg     *appconfig.Config
	}

	testCases := []struct {
		name string
		run  func(t *testing.T, suite *itf.Suite, cfg cfgBundle, sessionService *services.SessionService, browserSessions *services.BrowserSessionService)
	}{
		{
			name: "post deletes session and clears browser state",
			run: func(t *testing.T, suite *itf.Suite, cfg cfgBundle, sessionService *services.SessionService, _ *services.BrowserSessionService) {
				t.Helper()

				token := "logout-test-session-token"

				err := sessionService.Create(suite.Env().Ctx, &session.CreateDTO{
					UserID:    suite.Env().User.ID(),
					TenantID:  suite.Env().Tenant.ID,
					IP:        "127.0.0.1",
					UserAgent: "logout-test-agent",
					Token:     token,
				})
				require.NoError(t, err)

				response := suite.POST("/logout").
					Cookie(cfg.cookiesCfg.SID, token).
					Expect(t).
					Status(http.StatusSeeOther).
					RedirectTo("/login")

				require.Equal(t, "no-store, no-cache, must-revalidate, private", response.Header("Cache-Control"))
				require.Equal(t, "no-cache", response.Header("Pragma"))
				require.Equal(t, "0", response.Header("Expires"))
				require.Empty(t, response.Header("Clear-Site-Data"))

				respCookies := response.Cookies()
				require.NotEmpty(t, respCookies, "expected at least one Set-Cookie header")

				var deletedCookie *http.Cookie
				for _, cookie := range respCookies {
					if cookie.Name == cfg.cookiesCfg.SID {
						deletedCookie = cookie
						break
					}
				}

				require.NotNil(t, deletedCookie, "expected cleared session cookie to be present")
				require.Empty(t, deletedCookie.Value)
				require.Equal(t, cfg.cookiesCfg.SID, deletedCookie.Name)
				require.Equal(t, cfg.cookiesCfg.Domain, deletedCookie.Domain)
				require.Equal(t, "/", deletedCookie.Path)
				require.Equal(t, -1, deletedCookie.MaxAge)
				require.True(t, deletedCookie.HttpOnly)
				require.Equal(t, cfg.appCfg.IsProduction(), deletedCookie.Secure)
				require.Equal(t, http.SameSiteLaxMode, deletedCookie.SameSite)
				require.WithinDuration(t, time.Unix(0, 0), deletedCookie.Expires, time.Second)

				_, err = sessionService.GetByToken(suite.Env().Ctx, token)
				require.ErrorIs(t, err, persistence.ErrSessionNotFound)
			},
		},
		{
			name: "post removes only current account and activates remaining account",
			run: func(t *testing.T, suite *itf.Suite, cfg cfgBundle, sessionService *services.SessionService, browserSessions *services.BrowserSessionService) {
				t.Helper()
				firstToken := "logout-first-session-token"
				secondToken := "logout-second-session-token"
				for _, token := range []string{firstToken, secondToken} {
					require.NoError(t, sessionService.Create(suite.Env().Ctx, &session.CreateDTO{
						UserID: suite.Env().User.ID(), TenantID: suite.Env().Tenant.ID,
						IP: "127.0.0.1", UserAgent: "logout-test-agent", Token: token,
					}))
				}
				first, err := sessionService.GetBrowserSessionByToken(suite.Env().Ctx, firstToken)
				require.NoError(t, err)
				second, err := sessionService.GetBrowserSessionByToken(suite.Env().Ctx, secondToken)
				require.NoError(t, err)
				cookie, err := browserSessions.Add(suite.Env().Ctx, "", first)
				require.NoError(t, err)
				cookie, err = browserSessions.Add(suite.Env().Ctx, cookie.Value, second)
				require.NoError(t, err)

				response := suite.POST("/logout").
					Cookie(cfg.cookiesCfg.SID, cookie.Value).
					Expect(t).
					Status(http.StatusSeeOther).
					RedirectTo("/")

				updatedCookie := findResponseCookie(t, response, cfg.cookiesCfg.SID)
				require.NotEmpty(t, updatedCookie.Value)
				_, err = sessionService.GetBrowserSessionByToken(suite.Env().Ctx, secondToken)
				require.ErrorIs(t, err, persistence.ErrSessionNotFound)
				_, err = sessionService.GetBrowserSessionByToken(suite.Env().Ctx, firstToken)
				require.NoError(t, err)
			},
		},
		{
			name: "logout all clears browser state",
			run: func(t *testing.T, suite *itf.Suite, cfg cfgBundle, sessionService *services.SessionService, _ *services.BrowserSessionService) {
				t.Helper()
				token := "logout-all-session-token"
				require.NoError(t, sessionService.Create(suite.Env().Ctx, &session.CreateDTO{
					UserID: suite.Env().User.ID(), TenantID: suite.Env().Tenant.ID,
					IP: "127.0.0.1", UserAgent: "logout-test-agent", Token: token,
				}))
				response := suite.POST("/logout/all").
					Cookie(cfg.cookiesCfg.SID, token).
					Expect(t).
					Status(http.StatusSeeOther).
					RedirectTo("/login")
				require.Equal(t, `"cache", "cookies", "storage"`, response.Header("Clear-Site-Data"))
				require.Empty(t, findResponseCookie(t, response, cfg.cookiesCfg.SID).Value)
			},
		},
		{
			name: "get returns method not allowed",
			run: func(t *testing.T, suite *itf.Suite, _ cfgBundle, _ *services.SessionService, _ *services.BrowserSessionService) {
				t.Helper()

				response := suite.GET("/logout").
					Expect(t).
					Status(http.StatusMethodNotAllowed)

				require.Equal(t, http.MethodPost, response.Header("Allow"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
				PermissionSchema: defaults.PermissionSchema(),
			})).AsUser().Build()

			persistTestUser(t, suite.Env())

			httpCfg := itf.GetService[httpconfig.Config](suite.Env())
			cookiesCfg := itf.GetService[cookies.Config](suite.Env())
			appCfg := itf.GetService[appconfig.Config](suite.Env())
			cfg := cfgBundle{httpCfg: httpCfg, cookiesCfg: cookiesCfg, appCfg: appCfg}
			browserSessions := itf.GetService[services.BrowserSessionService](suite.Env())
			controller := controllers.NewLogoutController(httpCfg, browserSessions)
			suite.Register(controller)

			sessionService := itf.GetService[services.SessionService](suite.Env())

			tc.run(t, suite, cfg, sessionService, browserSessions)
		})
	}
}

func findResponseCookie(t *testing.T, response *itf.Response, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	require.FailNow(t, "response cookie not found", name)
	return nil
}
