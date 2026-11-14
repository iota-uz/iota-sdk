package controllers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antchfx/htmlquery"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/modules/logging/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/logging/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/fielddiff"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../../../../"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func newHistorySuite(t *testing.T, perms ...permission.Permission) *itf.Suite {
	t.Helper()
	suite := itf.NewSuiteBuilder(t).WithComponents(modules.Components()...).AsUser(perms...).Build()
	env := suite.Env()
	registry := itf.GetService[history.Registry](env)
	service := itf.GetService[history.Service](env)
	require.NotNil(t, registry)
	require.NotNil(t, service)
	require.NoError(t, registry.Register(history.Definition{
		Type:           "finance.payment.purpose_changed",
		Version:        1,
		Category:       businessevent.CategoryBusiness,
		TitleKey:       "Payments.History.PurposeChanged",
		DescriptionKey: "Payments.History.PurposeChangedSummary",
		Resources: []history.Resource{{Name: "payment", Fields: []history.Field{
			{Name: "purpose", LabelKey: "Payments.Fields.Purpose", Type: history.ValueString},
		}}},
	}))
	recordPurposeChange(t, suite, "104", "demo_op_104", time.Time{})
	suite.Register(controllers.NewHistoryController("", service, registry))
	return suite
}

// recordPurposeChange stores a payment purpose change by the test actor; a zero at means now.
func recordPurposeChange(t *testing.T, suite *itf.Suite, id, operation string, at time.Time) {
	t.Helper()
	env := suite.Env()
	registry := itf.GetService[history.Registry](env)
	email, err := internet.NewEmail("history-ui-actor@example.com")
	require.NoError(t, err)
	actor := user.New("Anna", "Primerova", email, user.UILanguageEN, user.WithID(42))
	ctx := history.WithExecution(composables.WithUser(env.Ctx, actor), history.Execution{Channel: businessevent.ChannelMCP, OperationID: operation})
	_, err = history.NewWriter(registry, persistence.NewBusinessEventRepository()).Record(ctx, history.Input{
		Type: "finance.payment.purpose_changed", Version: 1, Outcome: businessevent.OutcomeSucceeded, OccurredAt: at,
		Subjects: []businessevent.Subject{{Resource: "payment", ID: id}},
		Changes: []businessevent.Change{{Resource: "payment", ID: id, Fields: []fielddiff.Diff{
			{Field: "purpose", Before: fielddiff.Present(json.RawMessage(`"Service payment"`)), After: fielddiff.Present(json.RawMessage(`"Service payment for September"`))},
		}}},
	})
	require.NoError(t, err)
}

func TestHistoryController_RendersScopeAndChips(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)

	html := suite.GET("/admin/history?resource=payment&id=104&fb=1&f=channel:is:mcp&f=period:after:2026-09-01").Expect(t).Status(http.StatusOK).HTML()
	require.Equal(t, "payment #104", html.Element(`//h1`).Text())
	require.Equal(t, "104", html.Element(`//input[@name="id"]`).Attr("value"))
	require.Equal(t, "payment", html.Element(`//input[@name="resource"]`).Attr("value"))
	require.Equal(t, "channel:is:mcp", html.Element(`//input[@name="f"][@data-fb-chip="0"]`).Attr("value"))
	require.Equal(t, "period:after:2026-09-01", html.Element(`//input[@name="f"][@data-fb-chip="1"]`).Attr("value"))
	require.Equal(t, "1", html.Element(`//input[@name="fb"]`).Attr("value"))
	require.Contains(t, html.Element(`//div[@id="history-events"]`).Attr("hx-get"), "/admin/history/events?resource=payment&id=104&fb=1&f=channel:is:mcp")
	html.Element(`//select[@name="channel"]`).NotExists()
	html.Element(`//form[@action="/admin/history"]//button[@type="submit"]`).NotExists()
	require.Empty(t, html.Elements(`//*[@data-fb-label="Resource"]`), "an object page is pinned to its resource")

	suite.GET("/admin/history/events?resource=payment&id=104&fb=1&f=channel:is:mcp").Expect(t).Status(http.StatusOK).
		Contains(`data-state="ready"`).
		Contains("Primerova Anna").
		Contains("Service payment for September").
		Contains("demo_op_104")

	suite.GET("/admin/history/events?resource=payment&id=105").Expect(t).Status(http.StatusOK).Contains(`data-state="empty"`)
	suite.GET("/admin/history/events?resource=users").Expect(t).Status(http.StatusOK).Contains(`data-state="invalid"`)
	suite.GET("/admin/history/events?fb=1&f=resource:is:users").Expect(t).Status(http.StatusOK).Contains(`data-state="invalid"`)
}

func TestHistoryController_ListsFiltersWithTranslatedLabels(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)

	html := suite.GET("/admin/history?fb=1&f=resource:is:finance.expense,finance.payment").Expect(t).Status(http.StatusOK).HTML()
	for _, field := range []string{"Period", "Resource", "Action", "Channel", "Result"} {
		require.NotEmpty(t, html.Elements(`//*[@data-fb-label="`+field+`"]`), field)
	}
	require.Equal(t, "resource:is:finance.expense,finance.payment", html.Element(`//input[@name="f"][@data-fb-chip="0"]`).Attr("value"))
	require.Contains(t, html.Element(`//input[@name="f"][@data-fb-chip="0"]/following-sibling::div`).Text(), "Expenses, Payments")

	resources := html.Elements(`//option[starts-with(@value, "core.") or starts-with(@value, "finance.")][string-length(@value) - string-length(translate(@value, ".", "")) = 1]`)
	require.NotEmpty(t, resources)
	for _, option := range resources {
		label := strings.TrimSpace(htmlquery.InnerText(option))
		require.NotContains(t, label, ".", "%s is listed by its raw key", htmlquery.SelectAttr(option, "value"))
	}
	groups := html.Elements(`//option[@data-fb-header]`)
	require.NotEmpty(t, groups, "actions are grouped by resource")
	require.Equal(t, "Departments", strings.TrimSpace(htmlquery.InnerText(html.Elements(`//option[@value="__group:Departments"]`)[0])))
}

func TestHistoryController_LegacyResourceLinkBecomesAChip(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)

	html := suite.GET("/admin/history?resource=payment").Expect(t).Status(http.StatusOK).HTML()
	require.Equal(t, "resource:is:payment", html.Element(`//input[@name="f"][@data-fb-chip="0"]`).Attr("value"))
	suite.GET("/admin/history/events?resource=payment").Expect(t).Status(http.StatusOK).Contains(`data-state="ready"`)
	suite.GET("/admin/history/events?resource=payment&resource=ignored&fb=1&f=resource:is:payment,payment").Expect(t).Status(http.StatusOK).Contains(`data-state="ready"`)
}

func TestHistoryController_ShowsTimeInTheApplicationZone(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)
	tashkent, err := time.LoadLocation("Asia/Tashkent")
	require.NoError(t, err)
	suite.BeforeEach(func(ctx context.Context) context.Context { return composables.WithLocation(ctx, tashkent) })

	page := suite.GET("/admin/history/events?resource=payment&id=104").Expect(t).Status(http.StatusOK).HTML()
	shown := strings.TrimSpace(page.Element(`//time`).Text())
	datetime := page.Element(`//time`).Attr("datetime")
	occurred, err := time.Parse(time.RFC3339, datetime)
	require.NoError(t, err)
	require.Equal(t, occurred.In(tashkent).Format("2006-01-02 15:04"), shown)
	require.NotContains(t, shown, "UTC")
}

func TestHistoryController_FiltersByPeriodInTheApplicationZone(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)
	tashkent, err := time.LoadLocation("Asia/Tashkent")
	require.NoError(t, err)
	suite.BeforeEach(func(ctx context.Context) context.Context { return composables.WithLocation(ctx, tashkent) })

	// 2026-10-01T20:00Z is already October 2 in Tashkent (UTC+5) but still October 1 in UTC.
	recordPurposeChange(t, suite, "201", "op_201", time.Date(2026, 10, 1, 18, 59, 0, 0, time.UTC))
	recordPurposeChange(t, suite, "202", "op_202", time.Date(2026, 10, 1, 20, 0, 0, 0, time.UTC))
	recordPurposeChange(t, suite, "203", "op_203", time.Date(2026, 10, 2, 18, 59, 0, 0, time.UTC))
	recordPurposeChange(t, suite, "204", "op_204", time.Date(2026, 10, 2, 19, 0, 0, 0, time.UTC))

	suite.GET("/admin/history/events?fb=1&f=period:on:2026-10-02").Expect(t).Status(http.StatusOK).
		Contains(`data-state="ready"`).
		Contains("payment #202").
		Contains("payment #203").
		NotContains("payment #201").
		NotContains("payment #204").
		NotContains("payment #104")
	suite.GET("/admin/history/events?fb=1&f=period:between:2026-10-01,2026-10-01").Expect(t).Status(http.StatusOK).
		Contains("payment #201").
		NotContains("payment #202")
	suite.GET("/admin/history/events?fb=1&f=period:before:2000-01-01").Expect(t).Status(http.StatusOK).Contains(`data-state="empty"`)
}

func TestHistoryController_DeniesWithoutPermission(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t)

	suite.GET("/admin/history").Expect(t).Status(http.StatusForbidden)
	suite.GET("/admin/history/events").Expect(t).Status(http.StatusOK).
		Contains(`data-state="denied"`).
		NotContains("Service payment for September")
}
