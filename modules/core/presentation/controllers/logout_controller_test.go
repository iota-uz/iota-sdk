package controllers_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestLogoutController_DeletesSessionAndClearsBrowserState(t *testing.T) {
	t.Parallel()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	})).AsUser().Build()

	persistTestUser(t, suite.Env())

	controller := controllers.NewLogoutController()
	suite.Register(controller)

	config := configuration.Use()
	token := "logout-test-session-token"
	sessionService := itf.GetService[services.SessionService](suite.Env())

	err := sessionService.Create(suite.Env().Ctx, &session.CreateDTO{
		UserID:    suite.Env().User.ID(),
		TenantID:  suite.Env().Tenant.ID,
		IP:        "127.0.0.1",
		UserAgent: "logout-test-agent",
		Token:     token,
	})
	require.NoError(t, err)

	response := suite.GET("/logout").
		Cookie(config.SidCookieKey, token).
		Expect(t).
		Status(http.StatusSeeOther).
		RedirectTo("/login")

	require.Equal(t, "no-store, no-cache, must-revalidate, private", response.Header("Cache-Control"))
	require.Equal(t, "no-cache", response.Header("Pragma"))
	require.Equal(t, "0", response.Header("Expires"))
	require.Equal(t, `"cache", "cookies", "storage"`, response.Header("Clear-Site-Data"))

	deletedCookie := response.Cookies()[0]
	require.Equal(t, config.SidCookieKey, deletedCookie.Name)
	require.Equal(t, "", deletedCookie.Value)
	require.Equal(t, config.Domain, deletedCookie.Domain)
	require.Equal(t, "/", deletedCookie.Path)
	require.Equal(t, -1, deletedCookie.MaxAge)
	require.True(t, deletedCookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, deletedCookie.SameSite)
	require.WithinDuration(t, time.Unix(0, 0), deletedCookie.Expires, time.Second)

	_, err = sessionService.GetByToken(suite.Env().Ctx, token)
	require.Error(t, err)
}
