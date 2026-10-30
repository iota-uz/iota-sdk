package controllers_test

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

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
	email, err := internet.NewEmail("history-ui-actor@example.com")
	require.NoError(t, err)
	actor := user.New("Anna", "Primerova", email, user.UILanguageEN, user.WithID(42))
	ctx := history.WithExecution(composables.WithUser(env.Ctx, actor), history.Execution{Channel: businessevent.ChannelMCP, OperationID: "demo_op_104"})
	_, err = history.NewWriter(registry, persistence.NewBusinessEventRepository()).Record(ctx, history.Input{
		Type: "finance.payment.purpose_changed", Version: 1, Outcome: businessevent.OutcomeSucceeded,
		Subjects: []businessevent.Subject{{Resource: "payment", ID: "104"}},
		Changes: []businessevent.Change{{Resource: "payment", ID: "104", Fields: []fielddiff.Diff{
			{Field: "purpose", Before: fielddiff.Present(json.RawMessage(`"Service payment"`)), After: fielddiff.Present(json.RawMessage(`"Service payment for September"`))},
		}}},
	})
	require.NoError(t, err)
	suite.Register(controllers.NewHistoryController("", service, registry))
	return suite
}

func TestHistoryController_RendersScopeAndFilters(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t, permissions.ViewLogs)

	html := suite.GET("/admin/history?resource=payment&id=104&channel=mcp&from=2026-10-01").Expect(t).Status(http.StatusOK).HTML()
	require.Equal(t, "payment #104", html.Element(`//h1`).Text())
	require.Equal(t, "mcp", html.Element(`//select[@name="channel"]/option[@selected]`).Attr("value"))
	require.Equal(t, "2026-10-01", html.Element(`//input[@name="from"]`).Attr("value"))
	require.Equal(t, "104", html.Element(`//input[@name="id"]`).Attr("value"))
	require.Contains(t, html.Element(`//div[@id="history-events"]`).Attr("hx-get"), "/admin/history/events?resource=payment&id=104&channel=mcp")

	suite.GET("/admin/history/events?resource=payment&id=104&channel=mcp").Expect(t).Status(http.StatusOK).
		Contains(`data-state="ready"`).
		Contains("Primerova Anna").
		Contains("Service payment for September").
		Contains("demo_op_104")

	suite.GET("/admin/history/events?resource=payment&id=105").Expect(t).Status(http.StatusOK).Contains(`data-state="empty"`)
	suite.GET("/admin/history/events?resource=users").Expect(t).Status(http.StatusOK).Contains(`data-state="invalid"`)
}

func TestHistoryController_DeniesWithoutPermission(t *testing.T) {
	t.Parallel()
	suite := newHistorySuite(t)

	suite.GET("/admin/history").Expect(t).Status(http.StatusForbidden)
	suite.GET("/admin/history/events").Expect(t).Status(http.StatusOK).
		Contains(`data-state="denied"`).
		NotContains("Service payment for September")
}
