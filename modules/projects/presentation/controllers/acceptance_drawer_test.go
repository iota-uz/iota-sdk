package controllers_test

import (
	"context"
	"embed"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/acceptance"
	"github.com/iota-uz/iota-sdk/modules/projects/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

func drawerForm(number, amount, signedOn string) url.Values {
	form := actForm(number, amount, "USD")
	form.Set("SignedOn", signedOn)
	return form
}

func TestAcceptanceController_Drawer_ValidationErrorKeepsInput(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)

	form := actForm("A-1", "400", "EUR")
	form.Del("Date")
	html := suite.POST(acceptanceDrawerPath(p.ID())).Form(form).HTMX().Expect(t).Status(200).HTML()

	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))
	html.Element("//input[@name='Number' and @value='A-1']").Exists()
	html.Element("//input[@name='Amount' and @value='400']").Exists()
	html.Element("//select[@name='CurrencyCode']/option[@value='EUR' and @selected]").Exists()

	documents, err := itf.GetService[services.AcceptanceService](env).GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestAcceptanceController_Drawer_InvalidAmountShowsFieldError(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)

	html := suite.POST(acceptanceDrawerPath(p.ID())).Form(actForm("A-1", "ten", "USD")).HTMX().Expect(t).Status(200).HTML()

	require.Len(t, html.Elements("//small[@data-testid='field-error']"), 1)
	html.Element("//input[@name='Amount' and @value='ten']").Exists()

	documents, err := itf.GetService[services.AcceptanceService](env).GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestAcceptanceController_Drawer_UnknownKindIsRejected(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)

	form := actForm("A-1", "400", "USD")
	form.Set("Kind", "INVOICE")
	html := suite.POST(acceptanceDrawerPath(p.ID())).Form(form).HTMX().Expect(t).Status(200).HTML()
	require.Len(t, html.Elements("//small[@data-testid='field-error']"), 1)

	documents, err := itf.GetService[services.AcceptanceService](env).GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestAcceptanceController_Drawer_SigningDayDecidesStatus(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)
	revenueService := itf.GetService[services.RevenueService](env)

	response := suite.POST(acceptanceDrawerPath(p.ID())).Form(drawerForm("A-1", "400", "2026-09-25")).HTMX().Expect(t).Status(200)
	require.Empty(t, response.Body(), "a saved drawer is replaced by nothing, so it closes")
	require.Contains(t, response.Header("Hx-Trigger"), "iota:refresh")
	require.Contains(t, response.Header("Hx-Trigger"), "#project-acceptance-"+p.ID().String())

	documents, err := acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Len(t, documents, 1)
	document := documents[0]
	require.Equal(t, acceptance.StatusSigned, document.Status())
	require.Equal(t, time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC), document.SignedOn().UTC())

	revenue, err := revenueService.ProjectRevenue(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(40000), revenue[0].Accepted.Amount())

	// Clearing the signing day takes the amount out of revenue again.
	documentPath := fmt.Sprintf("%s/%s/drawer", acceptancePath(p.ID()), document.ID())
	suite.POST(documentPath).Form(drawerForm("A-1", "400", "")).HTMX().Expect(t).Status(200)
	draft, err := acceptanceService.GetByID(env.Ctx, document.ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.StatusDraft, draft.Status())
	require.True(t, draft.SignedOn().IsZero())

	revenue, err = revenueService.ProjectRevenue(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Equal(t, int64(0), revenue[0].Accepted.Amount())
}

func TestAcceptanceController_Drawer_EditUpdatesTheDocument(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	suite.POST(acceptanceDrawerPath(p.ID())).Form(drawerForm("A-1", "400", "")).HTMX().Expect(t).Status(200)
	documents, err := acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Len(t, documents, 1)
	documentPath := fmt.Sprintf("%s/%s/drawer", acceptancePath(p.ID()), documents[0].ID())

	html := suite.GET(documentPath).HTMX().Expect(t).Status(200).HTML()
	html.Element("//form[@hx-post='" + documentPath + "']").Exists()
	html.Element("//input[@name='Number' and @value='A-1']").Exists()
	html.Element("//select[@name='Kind']/option[@value='ACT' and @selected]").Exists()
	html.Element("//button[@data-test-id='drawer-delete']").Exists()
	html.Element("//div[@data-test-id='drawer-footer']").Exists()
	html.Element("//button[@data-test-id='drawer-close']").Exists()

	form := drawerForm("A-2", "1 500,50", "2026-09-25")
	form.Set("Kind", string(acceptance.KindProtocol))
	form.Set("Description", "Phase two")
	suite.POST(documentPath).Form(form).HTMX().Expect(t).Status(200)

	updated, err := acceptanceService.GetByID(env.Ctx, documents[0].ID())
	require.NoError(t, err)
	require.Equal(t, acceptance.KindProtocol, updated.Kind())
	require.Equal(t, "A-2", updated.Number())
	require.Equal(t, int64(150050), updated.Amount().Amount())
	require.Equal(t, "Phase two", updated.Description())
	require.Equal(t, acceptance.StatusSigned, updated.Status())
}

func TestAcceptanceController_Drawer_CancelledDocumentIsNotEditable(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	document, err := acceptanceService.Create(env.Ctx, acceptance.New(
		p.ID(), acceptance.KindAct, "A-1", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), money.New(40000, "USD"),
		acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)
	_, err = acceptanceService.Cancel(env.Ctx, p.ID(), document.ID())
	require.NoError(t, err)

	documentPath := fmt.Sprintf("%s/%s/drawer", acceptancePath(p.ID()), document.ID())
	suite.GET(documentPath).HTMX().Expect(t).Status(409)
	suite.POST(documentPath).Form(drawerForm("A-9", "900", "")).HTMX().Expect(t).Status(409)

	unchanged, err := acceptanceService.GetByID(env.Ctx, document.ID())
	require.NoError(t, err)
	require.Equal(t, "A-1", unchanged.Number())
}

func TestAcceptanceController_Drawer_Delete(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	document, err := acceptanceService.Create(env.Ctx, acceptance.New(
		p.ID(), acceptance.KindAct, "A-1", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), money.New(40000, "USD"),
		acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	response := suite.DELETE(fmt.Sprintf("%s/%s/drawer", acceptancePath(p.ID()), document.ID())).HTMX().Expect(t).Status(200)
	require.Empty(t, response.Body())
	require.Contains(t, response.Header("Hx-Trigger"), "iota:refresh")

	documents, err := acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Empty(t, documents)
}

func TestAcceptanceController_Drawer_DocumentOfAnotherProject(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t)
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	document, err := acceptanceService.Create(env.Ctx, acceptance.New(
		p.ID(), acceptance.KindAct, "A-1", time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), money.New(40000, "USD"),
		acceptance.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	otherPath := fmt.Sprintf("%s/%s/drawer", acceptancePath(uuid.New()), document.ID())
	suite.GET(otherPath).HTMX().Expect(t).Status(404)
	suite.DELETE(otherPath).HTMX().Expect(t).Status(404)
}

// extension is what an application does to plug into the form: its own
// currency rule, its own kinds, and its own reaction to a save.
type extension struct{}

func (extension) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "act-form-extension", Requires: []string{"projects"}}
}

func (extension) LocaleFS() []*embed.FS { return nil }

func (extension) Build(builder *composition.Builder) error {
	composition.Provide[services.AcceptanceKinds](builder, kinds{})
	composition.Provide[services.AcceptanceCurrencyPolicy](builder, lockedCurrency{})
	composition.Provide[controllers.AcceptanceSaveHook](builder, redirectToProject{})
	return nil
}

type kinds struct{}

func (kinds) Kinds(context.Context) []acceptance.KindOption {
	return []acceptance.KindOption{
		{Value: acceptance.KindAct},
		{Value: "COMPLETION", Label: "Completion certificate"},
	}
}

type lockedCurrency struct{}

func (lockedCurrency) ForProject(context.Context, uuid.UUID) (services.AcceptanceCurrency, error) {
	return services.AcceptanceCurrency{Code: "EUR", Locked: true}, nil
}

type redirectToProject struct{}

func (redirectToProject) Saved(_ context.Context, outcome controllers.AcceptanceOutcome) controllers.AcceptanceReaction {
	return controllers.AcceptanceReaction{Redirect: "/erp/projects/" + outcome.ProjectID.String()}
}

func TestAcceptanceController_Drawer_ApplicationExtensionPoints(t *testing.T) {
	t.Parallel()
	suite, env, p := newAcceptanceSuite(t, extension{})
	acceptanceService := itf.GetService[services.AcceptanceService](env)

	html := suite.GET(acceptanceDrawerPath(p.ID())).HTMX().Expect(t).Status(200).HTML()
	html.Element("//select[@name='Kind']/option[@value='COMPLETION']").Exists()
	require.Empty(t, html.Elements("//select[@name='Kind']/option[@value='PROTOCOL']"))
	html.Element("//input[@type='hidden' and @name='CurrencyCode' and @value='EUR']").Exists()
	require.Empty(t, html.Elements("//select[@name='CurrencyCode']"))

	// The locked currency wins over what the browser posted.
	form := actForm("C-1", "400", "USD")
	form.Set("Kind", "COMPLETION")
	response := suite.POST(acceptanceDrawerPath(p.ID())).Form(form).HTMX().Expect(t).Status(200)
	require.Equal(t, "/erp/projects/"+p.ID().String(), response.Header("Hx-Redirect"))

	documents, err := acceptanceService.GetByProjectID(env.Ctx, p.ID())
	require.NoError(t, err)
	require.Len(t, documents, 1)
	require.Equal(t, acceptance.Kind("COMPLETION"), documents[0].Kind())
	require.Equal(t, "EUR", documents[0].Amount().Currency().Code)

	// The panel names a kind by the label the application gave it.
	suite.GET(acceptancePath(p.ID())).HTMX().Expect(t).Status(200).Contains("Completion certificate")
}
