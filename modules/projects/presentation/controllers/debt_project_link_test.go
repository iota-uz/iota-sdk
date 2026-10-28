package controllers_test

import (
	"net/url"
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	financepermissions "github.com/iota-uz/iota-sdk/modules/finance/permissions"
	financecontrollers "github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	financeServices "github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/modules/projects"
	"github.com/iota-uz/iota-sdk/modules/projects/domain/aggregates/project"
	"github.com/iota-uz/iota-sdk/modules/projects/permissions"
	"github.com/iota-uz/iota-sdk/modules/projects/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestDebtForm_LinksProject(t *testing.T) {
	t.Parallel()
	user := itf.User(
		permissions.ProjectRead,
		financepermissions.DebtCreate,
		financepermissions.DebtRead,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent(), projects.NewComponent()).Build().
		AsUser(user)
	env := suite.Environment()

	require.NoError(t, corepersistence.NewCurrencyRepository().Create(env.Ctx, currency.USD))

	directory, err := composition.Resolve[financeServices.ProjectDirectory](env.Container)
	require.NoError(t, err)
	suite.Register(financecontrollers.NewDebtsController(
		itf.GetService[financeServices.DebtService](env),
		itf.GetService[financeServices.CounterpartyService](env),
		itf.GetService[financeServices.MoneyAccountService](env),
		itf.GetService[coreservices.CurrencyService](env),
		directory,
	))

	createdCounterparty, err := itf.GetService[financeServices.CounterpartyService](env).Create(env.Ctx, counterparty.New(
		"Contractor",
		counterparty.Supplier,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	))
	require.NoError(t, err)

	renovation := project.New("Office renovation", createdCounterparty.ID(), project.WithTenantID(env.Tenant.ID))
	require.NoError(t, itf.GetService[services.ProjectService](env).Create(env.Ctx, renovation))

	suite.GET("/finance/debts/new/drawer").
		Expect(t).
		Status(200).
		HTML().
		Element("//select[@name='ProjectID']/option[text()='Office renovation']").
		Exists()

	form := url.Values{}
	form.Set("CounterpartyID", createdCounterparty.ID().String())
	form.Set("Type", "PAYABLE")
	form.Set("Amount", "120.00")
	form.Set("CurrencyCode", "USD")
	form.Set("ProjectID", renovation.ID().String())
	form.Set("Description", "Materials")
	suite.POST("/finance/debts").
		Form(form).
		HTMX().
		HTMXTarget("debt-create-drawer").
		Expect(t).
		Status(200)

	debts, err := itf.GetService[financeServices.DebtService](env).GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, debts, 1)
	require.NotNil(t, debts[0].ProjectID())
	require.Equal(t, renovation.ID(), *debts[0].ProjectID())
}
