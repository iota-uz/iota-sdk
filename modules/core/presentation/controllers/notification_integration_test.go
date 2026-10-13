package controllers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestNotificationController_PersistedInbox(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser().Build()
	persistAdministrativeTestActor(t, suite)
	suite.Register(controllers.NewNotificationController())
	ctx := suite.Env().Ctx
	repo := persistence.NewNotificationRepository()
	var first notification.Notification
	for i := range 21 {
		n, err := notification.New(suite.Env().User.ID(), fmt.Sprintf("Inbox item %02d", i), "Safe <script> payload", notification.WithActionURL("/users"))
		require.NoError(t, err)
		n, err = repo.Create(ctx, n)
		require.NoError(t, err)
		if i == 0 {
			first = n
		}
	}
	email, err := internet.NewEmail("other@example.com")
	require.NoError(t, err)
	userRepo := persistence.NewUserRepository(persistence.NewUploadRepository())
	other, err := userRepo.Create(ctx, user.New("Other", "User", email, user.UILanguageEN, user.WithTenantID(suite.Env().TenantID())))
	require.NoError(t, err)
	foreign, err := notification.New(other.ID(), "Other user private", "Secret")
	require.NoError(t, err)
	foreign, err = repo.Create(ctx, foreign)
	require.NoError(t, err)
	otherTenant, err := persistence.NewTenantRepository().Create(ctx, tenant.New("Other tenant"))
	require.NoError(t, err)
	foreignCtx := composables.WithTenantID(ctx, otherTenant.ID())
	email, err = internet.NewEmail("foreign@example.com")
	require.NoError(t, err)
	foreignUser, err := userRepo.Create(foreignCtx, user.New("Foreign", "User", email, user.UILanguageEN, user.WithTenantID(otherTenant.ID())))
	require.NoError(t, err)
	foreignTenantNotification, err := notification.New(foreignUser.ID(), "Other tenant private", "Secret")
	require.NoError(t, err)
	foreignTenantNotification, err = repo.Create(foreignCtx, foreignTenantNotification)
	require.NoError(t, err)
	body := suite.GET("/notifications").Expect(t).Status(http.StatusOK).Body()
	require.Contains(t, body, "notification-bell")
	require.Equal(t, 20, strings.Count(body, `data-testid="notification"`))
	require.NotContains(t, body, "Other user private")
	require.NotContains(t, body, "Other tenant private")
	require.Contains(t, body, "Safe &lt;script&gt; payload")
	require.Contains(t, body, `href="/users"`)
	pageItems, err := repo.List(ctx, suite.Env().User.ID(), notification.FindParams{Limit: 20})
	require.NoError(t, err)
	require.Len(t, pageItems, 20)
	body = suite.GET("/notifications?cursor=" + notification.CursorFor(pageItems[19])).HTMX().Expect(t).Status(http.StatusOK).Body()
	require.Equal(t, 1, strings.Count(body, `data-testid="notification"`))
	require.Contains(t, suite.GET("/notifications/summary").Expect(t).Status(http.StatusOK).Body(), ">21</span>")
	suite.POST("/notifications/" + first.ID().String() + "/read").HTMX().Expect(t).Status(http.StatusNoContent)
	count, err := repo.UnreadCount(ctx, suite.Env().User.ID())
	require.NoError(t, err)
	require.EqualValues(t, 20, count)
	body = suite.GET("/notifications?unread=true").HTMX().Expect(t).Status(http.StatusOK).Body()
	require.NotContains(t, body, first.Title())
	for _, id := range []string{foreign.ID().String(), foreignTenantNotification.ID().String()} {
		suite.POST("/notifications/" + id + "/read").HTMX().Expect(t).Status(http.StatusNotFound)
	}
	suite.POST("/notifications/read-all").HTMX().Expect(t).Status(http.StatusNoContent)
	count, err = repo.UnreadCount(ctx, suite.Env().User.ID())
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = repo.UnreadCount(ctx, other.ID())
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	count, err = repo.UnreadCount(foreignCtx, foreignUser.ID())
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	require.Empty(t, strings.TrimSpace(suite.GET("/notifications/summary").Expect(t).Status(http.StatusOK).Body()))
	require.NotContains(t, suite.GET("/notifications?unread=true").HTMX().Expect(t).Status(http.StatusOK).Body(), `data-testid="notification"`)
}
