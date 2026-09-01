package controllers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/headers"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func TestLoginController_CustomRendererReceivesAccountPickerContext_Scenarios(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		nextURL string
	}{
		{name: "renders and activates a stored account", nextURL: "/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
				PermissionSchema: defaults.PermissionSchema(),
			})).AsUser().Build()
			persistTestUser(t, suite.Env())

			sessionService := itf.GetService[services.SessionService](suite.Env())
			browserSessions := itf.GetService[services.BrowserSessionService](suite.Env())
			token := "login-renderer-" + uuid.NewString()
			require.NoError(t, sessionService.Create(suite.Env().Ctx, &session.CreateDTO{
				Token: token, UserID: suite.Env().User.ID(), TenantID: suite.Env().Tenant.ID,
				IP: "127.0.0.1", UserAgent: "login-renderer-test",
			}))
			sess, err := sessionService.GetBrowserSessionByToken(suite.Env().Ctx, token)
			require.NoError(t, err)
			browserCookie, err := browserSessions.Add(suite.Env().Ctx, "", sess)
			require.NoError(t, err)

			var captured controllers.LoginPageViewModel
			options := &controllers.LoginControllerOptions{
				Renderer: func(_ context.Context, vm controllers.LoginPageViewModel) templ.Component {
					captured = vm
					return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
						_, err := fmt.Fprintf(w, "accounts=%d next=%s ref=%s", len(vm.Accounts), vm.NextURL, vm.Accounts[0].SessionReference)
						return err
					})
				},
			}
			controller := controllers.NewLoginControllerWithBrowserSessions(
				itf.GetService[services.AuthService](suite.Env()),
				itf.GetService[services.AuthFlowService](suite.Env()),
				browserSessions,
				itf.GetService[httpconfig.Config](suite.Env()),
				itf.GetService[cookies.Config](suite.Env()),
				itf.GetService[headers.Config](suite.Env()),
				itf.GetService[googleoauthconfig.Config](suite.Env()),
				options,
			)
			suite.Register(controller)

			cookiesCfg := itf.GetService[cookies.Config](suite.Env())
			response := suite.GET("/login?next="+tt.nextURL).
				Cookie(cookiesCfg.SID, browserCookie.Value).
				Expect(t).
				Status(http.StatusOK)

			require.Len(t, captured.Accounts, 1)
			assert.Equal(t, tt.nextURL, captured.NextURL)
			assert.Equal(t, suite.Env().User.ID(), captured.Accounts[0].UserID)
			assert.Equal(t, suite.Env().Tenant.ID.String(), captured.Accounts[0].TenantID)
			assert.NotEqual(t, token, captured.Accounts[0].SessionReference)
			assert.Contains(t, response.Body(), "accounts=1 next="+tt.nextURL)

			suite.POST("/login/session?next="+tt.nextURL).
				Cookie(cookiesCfg.SID, browserCookie.Value).
				FormString("SessionReference", captured.Accounts[0].SessionReference).
				Expect(t).
				Status(http.StatusSeeOther).
				RedirectTo(tt.nextURL)
		})
	}
}
