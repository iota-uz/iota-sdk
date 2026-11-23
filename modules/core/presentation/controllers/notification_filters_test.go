package controllers_test

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// TestNotificationController_Index_FilterChips covers the chip bar of the
// notification center: read state, importance and arrival date narrow the
// list in the query, the former ?unread=true link becomes a chip, htmx
// answers carry the builder out-of-band and cursor links keep the chips.
func TestNotificationController_Index_FilterChips(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser().Build()
	persistAdministrativeTestActor(t, suite)
	suite.Register(controllers.NewNotificationController())
	ctx := suite.Env().Ctx
	repo := persistence.NewNotificationRepository()
	userID := suite.Env().User.ID()
	create := func(title string, opts ...notification.Option) notification.Notification {
		t.Helper()
		n, err := notification.New(userID, title, "", opts...)
		require.NoError(t, err)
		n, err = repo.Create(ctx, n)
		require.NoError(t, err)
		return n
	}
	read := create("Read info note", notification.WithLevel(notification.LevelInfo))
	require.NoError(t, repo.MarkRead(ctx, userID, read.ID()))
	create("Unread error alert", notification.WithLevel(notification.LevelError))
	create("Old warning", notification.WithLevel(notification.LevelWarning), notification.WithCreatedAt(time.Date(2020, 1, 15, 12, 0, 0, 0, time.UTC)))

	t.Run("Level chip narrows the list and renders the builder", func(t *testing.T) {
		body := suite.GET("/notifications?fb=1&f=level:is:error").Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, `x-data="filterBuilder"`)
		require.Contains(t, body, `value="level:is:error"`)
		require.Contains(t, body, "Unread error alert")
		require.NotContains(t, body, "Read info note")
		require.NotContains(t, body, "Old warning")
	})

	t.Run("Unread is-false chip lists only read notifications", func(t *testing.T) {
		body := suite.GET("/notifications?fb=1&f=unread:is:false").Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, "Read info note")
		require.NotContains(t, body, "Unread error alert")
	})

	t.Run("Legacy unread link maps to the unread chip", func(t *testing.T) {
		body := suite.GET("/notifications?unread=true").Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, `value="unread:is:true"`)
		require.Contains(t, body, "Unread error alert")
		require.Contains(t, body, "Old warning")
		require.NotContains(t, body, "Read info note")
	})

	t.Run("Created date chip", func(t *testing.T) {
		body := suite.GET("/notifications?fb=1&f=created_at:before:2021-01-01").Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, "Old warning")
		require.NotContains(t, body, "Unread error alert")
		require.NotContains(t, body, "Read info note")
	})

	t.Run("Htmx answer re-renders the list with the builder out-of-band", func(t *testing.T) {
		body := suite.GET("/notifications?fb=1&f=level:is:warning").HTMX().Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, `data-testid="notification-center"`)
		require.Contains(t, body, `hx-swap-oob="outerHTML"`)
		require.Contains(t, body, `id="notification-filters"`)
		require.Contains(t, body, "Old warning")
		require.NotContains(t, body, "Unread error alert")
		require.NotContains(t, body, "<html")
	})

	t.Run("Cursor links keep the chips", func(t *testing.T) {
		for i := range 21 {
			create(fmt.Sprintf("Paged success %02d", i), notification.WithLevel(notification.LevelSuccess))
		}
		body := suite.GET("/notifications?fb=1&f=level:is:success").Expect(t).Status(http.StatusOK).Body()
		require.Equal(t, 20, strings.Count(body, `data-testid="notification"`))
		next := regexp.MustCompile(`href="(/notifications\?[^"]*cursor=[^"]+)"`).FindStringSubmatch(body)
		require.NotNil(t, next, "next page link")
		link := html.UnescapeString(next[1])
		require.Contains(t, link, "f=level%3Ais%3Asuccess")
		require.Contains(t, link, "fb=1")
		page2 := suite.GET(link).Expect(t).Status(http.StatusOK).Body()
		require.Equal(t, 1, strings.Count(page2, `data-testid="notification"`))
		require.Contains(t, page2, "Paged success 00")
		require.NotContains(t, page2, "Unread error alert")
	})

	t.Run("history restore gets the whole page", func(t *testing.T) {
		body := suite.GET("/notifications?f=level:is:success&fb=1").HTMX().HTMXHistoryRestore().Expect(t).Status(http.StatusOK).Body()
		require.Contains(t, body, "<html")
		require.Contains(t, body, `x-data="filterBuilder"`)
	})
}
