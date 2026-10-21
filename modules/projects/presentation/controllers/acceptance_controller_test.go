package controllers_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financecontrollers "github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	financeServices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func newAcceptanceSuite(t *testing.T) (*itf.Suite, *itf.TestEnvironment, project.Project) {
	t.Helper()
	suite := newProjectStageSuiteBuilder(t).Build().AsUser(itf.User())
	suite.Register(controllers.NewAcceptanceController())
	env := suite.Environment()

	currencyService := itf.GetService[coreservices.CurrencyService](env)
	for _, c := range []currency.Currency{currency.USD, currency.EUR} {
		require.NoError(t, currencyService.Create(env.Ctx, &currency.CreateDTO{
			Code:   string(c.Code()),
			Name:   c.Name(),
			Symbol: string(c.Symbol()),
		}))
	}

	client, err := itf.GetService[financeServices.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"Acceptance Client",
		counterparty.Customer,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	p := project.New(
		"Acceptance Project",
		client.ID(),
		project.WithTenantID(env.Tenant.ID),
		project.WithContract(money.New(100000, "USD")),
	)
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(env.Ctx, p))
	return suite, env, p
}

func acceptancePath(projectID uuid.UUID) string {
	return fmt.Sprintf("/projects/%s/acceptance", projectID)
}

func actForm(number, amount, currencyCode string) url.Values {
	form := url.Values{}
	form.Set("Kind", string(acceptance.KindAct))
	form.Set("Number", number)
	form.Set("Date", "2026-09-20")
	form.Set("Amount", amount)
	form.Set("CurrencyCode", currencyCode)
	return form
}

func TestAcceptanceController_Panel_DefaultsToContractCurrency(t *testing.T) {
	t.Parallel()
	suite, _, p := newAcceptanceSuite(t)

	html := suite.GET(acceptancePath(p.ID())).HTMX().Expect(t).Status(200).HTML()
	html.Element("//select[@name='CurrencyCode']/option[@value='USD' and @selected]").Exists()
	html.Element("//form[@hx-post='" + acceptancePath(p.ID()) + "']").Exists()
}

func TestAcceptanceController_Create_ValidationErrorKeepsInput(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)

	form := actForm("A-1", "400", "EUR")
	form.Del("Date")
	html := suite.POST(acceptancePath(p.ID())).Form(form).HTMX().Expect(t).Status(200).HTML()

	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))
	html.Element("//input[@name='Number' and @value='A-1']").Exists()
	html.Element("//input[@name='Amount' and @value='400']").Exists()
	html.Element("//select[@name='CurrencyCode']/option[@value='EUR' and @selected]").Exists()

	documents, err := itf.GetService[services.AcceptanceService](env).GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestAcceptanceController_Lifecycle(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)
	revenueService := itf.GetService[services.RevenueService](env)

	suite.POST(acceptancePath(p.ID())).Form(actForm("A-1", "400", "USD")).HTMX().
		Expect(t).Status(200).Contains("A-1")

	documents, err := acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Len(t, documents, 1)
	document := documents[0]
	require.Equal(t, acceptance.KindAct, document.Kind())
	require.Equal(t, acceptance.StatusDraft, document.Status())
	require.Equal(t, int64(40000), document.Amount().Amount())
	require.Equal(t, "USD", document.Amount().Currency().Code)
	require.Equal(t, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), document.Date().UTC())

	revenue, err := revenueService.ProjectRevenue(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Len(t, revenue, 1)
	require.Equal(t, int64(0), revenue[0].Accepted.Amount())

	documentPath := fmt.Sprintf("%s/%s", acceptancePath(p.ID()), document.ID())
	suite.POST(documentPath + "/sign").HTMX().Expect(t).Status(200)
	signed, err := acceptanceService.GetByID(env.Ctx, document.ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.StatusSigned, signed.Status())

	revenue, err = revenueService.ProjectRevenue(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(100000), revenue[0].Contract.Amount())
	require.Equal(t, int64(40000), revenue[0].Accepted.Amount())
	require.Equal(t, int64(60000), revenue[0].Open.Amount())

	suite.POST(documentPath + "/sign").HTMX().Expect(t).Status(409)

	suite.POST(documentPath + "/cancel").HTMX().Expect(t).Status(200)
	cancelled, err := acceptanceService.GetByID(env.Ctx, document.ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.StatusCancelled, cancelled.Status())

	revenue, err = revenueService.ProjectRevenue(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(0), revenue[0].Accepted.Amount())
	require.Equal(t, int64(100000), revenue[0].Open.Amount())

	suite.DELETE(documentPath).HTMX().Expect(t).Status(200).NotContains("A-1")
	documents, err = acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestCounterpartiesController_ShowsClientRevenue(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	suite.Register(financecontrollers.NewCounterpartiesController(itf.GetService[financeServices.CounterpartyService](env)))

	document, err := itf.GetService[services.AcceptanceService](env).Create(env.Ctx, acceptance.New(
		p.ID(),
		acceptance.KindAct,
		"A-7",
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		money.New(25000, "EUR"),
		acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	_, err = itf.GetService[services.AcceptanceService](env).Sign(env.Ctx, p.ID(), document.ID())
	require.NoError(t, err)

	suite.GET(fmt.Sprintf("/finance/counterparties/%s", p.CounterpartyID())).
		Expect(t).
		Status(200).
		Contains("$1,000.00").
		Contains("€250.00").
		Contains("EUR").
		Contains("USD")
}

func TestAcceptanceController_DocumentOfAnotherProject(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	other := project.New("Other Project", p.CounterpartyID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(env.Ctx, other))
	document, err := acceptanceService.Create(env.Ctx, acceptance.New(
		other.ID(),
		acceptance.KindAct,
		"B-1",
		time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		money.New(10000, "USD"),
		acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	documentPath := fmt.Sprintf("%s/%s", acceptancePath(p.ID()), document.ID())
	suite.POST(documentPath + "/sign").HTMX().Expect(t).Status(404)
	suite.POST(documentPath + "/cancel").HTMX().Expect(t).Status(404)
	suite.DELETE(documentPath).HTMX().Expect(t).Status(404)

	unchanged, err := acceptanceService.GetByID(env.Ctx, document.ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.StatusDraft, unchanged.Status())
}

func TestProjectController_Update_Contract(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	suite.Register(controllers.NewProjectController())
	projectService := itf.GetService[services.ProjectService](env)

	form := url.Values{}
	form.Set("Name", p.Name())
	form.Set("CounterpartyID", p.CounterpartyID().String())
	form.Set("ContractAmount", "2500.50")
	form.Set("ContractCurrency", "")

	html := suite.POST(fmt.Sprintf("%s/%s", ProjectBasePath, p.ID())).
		Form(form).
		Header("HX-Target", "project-edit-drawer").
		Expect(t).
		Status(200).
		HTML()
	require.Len(t, html.Elements("//small[@data-testid='field-error']"), 1)
	html.Element("//input[@name='ContractAmount' and @value='2500.5']").Exists()

	unchanged, err := projectService.GetByID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(100000), unchanged.Contract().Amount())

	form.Set("ContractCurrency", "EUR")
	suite.POST(fmt.Sprintf("%s/%s", ProjectBasePath, p.ID())).
		Form(form).
		Expect(t).
		Status(302)

	updated, err := projectService.GetByID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(250050), updated.Contract().Amount())
	require.Equal(t, "EUR", updated.Contract().Currency().Code)

	suite.GET(fmt.Sprintf("%s/%s/drawer", ProjectBasePath, p.ID())).
		Expect(t).
		Status(200).
		HTML().
		Element("//input[@name='ContractAmount' and @value='2500.5']").Exists()

	form.Set("ContractAmount", "")
	form.Set("ContractCurrency", "")
	suite.POST(fmt.Sprintf("%s/%s", ProjectBasePath, p.ID())).
		Form(form).
		Expect(t).
		Status(302)

	cleared, err := projectService.GetByID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Nil(t, cleared.Contract())
}
