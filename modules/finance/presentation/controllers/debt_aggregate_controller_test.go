package controllers_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	debtAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/stretchr/testify/require"
)

var (
	DebtAggregateBasePath = "/finance/debt-aggregates"
)

func TestDebtAggregateController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtService := itf.GetService[services.DebtService](env)
	counterpartyService := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtService, counterpartyService)
	suite.Register(controller)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Test Debt Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debts
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(500.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Test receivable debt"),
	)

	debt2 := debtAggregate.New(
		debtAggregate.DebtTypePayable,
		money.NewFromFloat(300.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Test payable debt"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)
	_, err = debtService.Create(env.Ctx, debt2)
	require.NoError(t, err)

	response := suite.GET(DebtAggregateBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 1)

	response.Contains("Test Debt Counterparty").
		Contains("500.00").
		Contains("300.00")
}

func TestDebtAggregateController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtService := itf.GetService[services.DebtService](env)
	counterpartyService := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtService, counterpartyService)
	suite.Register(controller)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"HTMX Test Counterparty",
		counterparty.Supplier,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(750.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("HTMX test debt"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	suite.GET(DebtAggregateBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Counterparty").
		Contains("750.00")
}

func TestDebtAggregateController_List_EmptyResult(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtSvc := itf.GetService[services.DebtService](env)
	counterpartySvc := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtSvc, counterpartySvc)
	suite.Register(controller)

	response := suite.GET(DebtAggregateBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	// Should have empty state row (no data rows)
	// The table scaffold renders an empty state <tr> when there's no data
	rows := html.Elements("//table//tbody//tr")
	require.Len(t, rows, 1, "Should have exactly 1 row (the empty state row)")
	// Verify it's the empty state, not actual data
	response.Contains("No data available")
}

func TestDebtAggregateController_GetCounterpartyDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtService := itf.GetService[services.DebtService](env)
	counterpartyService := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtService, counterpartyService)
	suite.Register(controller)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Drawer Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debts for this counterparty
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(100.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Drawer test receivable"),
	)

	debt2 := debtAggregate.New(
		debtAggregate.DebtTypePayable,
		money.NewFromFloat(50.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Drawer test payable"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)
	_, err = debtService.Create(env.Ctx, debt2)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s/drawer", DebtAggregateBasePath, createdCounterparty.ID().String())).
		Expect(t).
		Status(200)

	response.Contains("Drawer Test Counterparty").
		Contains("Drawer test receivable").
		Contains("Drawer test payable").
		Contains("100.00").
		Contains("50.00")
}

func TestDebtAggregateController_GetCounterpartyDrawer_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtSvc := itf.GetService[services.DebtService](env)
	counterpartySvc := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtSvc, counterpartySvc)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s/drawer", DebtAggregateBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestDebtAggregateController_GetCounterpartyDrawer_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtSvc := itf.GetService[services.DebtService](env)
	counterpartySvc := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtSvc, counterpartySvc)
	suite.Register(controller)

	suite.GET(DebtAggregateBasePath + "/invalid-uuid/drawer").
		Expect(t).
		Status(404)
}

func TestDebtAggregateController_Permission_Forbidden(t *testing.T) {
	t.Parallel()
	userWithoutPermission := itf.User()

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(userWithoutPermission)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtSvc := itf.GetService[services.DebtService](env)
	counterpartySvc := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtSvc, counterpartySvc)
	suite.Register(controller)

	suite.GET(DebtAggregateBasePath).
		Expect(t).
		Status(403).
		Contains("forbidden")
}

func TestDebtAggregateController_MultipleCounterparties(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD)

	debtService := itf.GetService[services.DebtService](env)
	counterpartyService := itf.GetService[services.CounterpartyService](env)

	controller := controllers.NewDebtAggregateController(debtService, counterpartyService)
	suite.Register(controller)

	// Create multiple counterparties
	counterparty1 := counterparty.New(
		"First Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	counterparty2 := counterparty.New(
		"Second Counterparty",
		counterparty.Supplier,
		counterparty.LLC,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty1, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)
	createdCounterparty2, err := counterpartyService.Create(env.Ctx, counterparty2)
	require.NoError(t, err)

	// Create debts for each counterparty
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(200.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty1.ID()),
		debtAggregate.WithDescription("First counterparty debt"),
	)

	debt2 := debtAggregate.New(
		debtAggregate.DebtTypePayable,
		money.NewFromFloat(150.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty2.ID()),
		debtAggregate.WithDescription("Second counterparty debt"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)
	_, err = debtService.Create(env.Ctx, debt2)
	require.NoError(t, err)

	response := suite.GET(DebtAggregateBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("First Counterparty").
		Contains("Second Counterparty").
		Contains("200.00").
		Contains("150.00")
}

func TestDebtAggregateController_List_FilterChips(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.NewSuiteBuilder(t).WithComponents(core.NewComponent(&core.ModuleOptions{
		PermissionSchema: defaults.PermissionSchema(),
	}), finance.NewComponent()).Build().
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env, currency.USD, currency.EUR)

	debtService := itf.GetService[services.DebtService](env)
	counterpartyService := itf.GetService[services.CounterpartyService](env)
	suite.Register(controllers.NewDebtAggregateController(debtService, counterpartyService))

	newCounterparty := func(name string) counterparty.Counterparty {
		cp, err := counterpartyService.Create(env.Ctx, counterparty.New(
			name, counterparty.Customer, counterparty.LLC, counterparty.WithTenantID(env.Tenant.ID),
		))
		require.NoError(t, err)
		return cp
	}
	addDebt := func(cp counterparty.Counterparty, typ debtAggregate.DebtType, amount float64, code string, settled bool) {
		d := debtAggregate.New(typ, money.NewFromFloat(amount, code),
			debtAggregate.WithTenantID(env.Tenant.ID),
			debtAggregate.WithCounterpartyID(cp.ID()),
		)
		if settled {
			d = d.UpdateOutstandingAmount(money.New(0, code))
		}
		_, err := debtService.Create(env.Ctx, d)
		require.NoError(t, err)
	}

	// Alpha owes us 700 and is owed 50, in dollars; Beta is owed by us in
	// euros; Gamma's only debt is settled.
	alpha := newCounterparty("Chip Alpha LLC")
	beta := newCounterparty("Chip Beta LLC")
	gamma := newCounterparty("Chip Gamma LLC")
	addDebt(alpha, debtAggregate.DebtTypeReceivable, 700, "USD", false)
	addDebt(alpha, debtAggregate.DebtTypePayable, 50, "USD", false)
	addDebt(beta, debtAggregate.DebtTypePayable, 400, "EUR", false)
	addDebt(gamma, debtAggregate.DebtTypeReceivable, 90, "USD", true)

	// Names also show as chip options, so a row is told by its drawer link.
	row := func(cp counterparty.Counterparty) string {
		return fmt.Sprintf("%s/%s/drawer", DebtAggregateBasePath, cp.ID())
	}
	cases := []struct {
		name    string
		query   string
		present []counterparty.Counterparty
		absent  []counterparty.Counterparty
	}{
		{"counterparty", "?f=counterparty:is:" + beta.ID().String(), []counterparty.Counterparty{beta}, []counterparty.Counterparty{alpha, gamma}},
		{"currency", "?f=currency:is:USD", []counterparty.Counterparty{alpha, gamma}, []counterparty.Counterparty{beta}},
		{"payable side", "?f=type:is:PAYABLE", []counterparty.Counterparty{alpha, beta}, []counterparty.Counterparty{gamma}},
		{"has outstanding", "?f=has_outstanding:is:true", []counterparty.Counterparty{alpha, beta}, []counterparty.Counterparty{gamma}},
		{"nothing outstanding", "?f=has_outstanding:is:false", []counterparty.Counterparty{gamma}, []counterparty.Counterparty{alpha, beta}},
		{"currency and outstanding", "?f=currency:is:USD&f=has_outstanding:is:true", []counterparty.Counterparty{alpha}, []counterparty.Counterparty{beta, gamma}},
		// A garbled id in the URL matches nothing instead of failing the query.
		{"malformed counterparty id", "?f=counterparty:is:not-a-uuid", nil, []counterparty.Counterparty{alpha, beta, gamma}},
		// An id typed in uppercase still finds its counterparty.
		{"uppercase counterparty id", "?f=counterparty:is:" + strings.ToUpper(beta.ID().String()), []counterparty.Counterparty{beta}, []counterparty.Counterparty{alpha, gamma}},
	}
	for _, tc := range cases {
		response := suite.GET(DebtAggregateBasePath + tc.query).HTMX().Expect(t).Status(200)
		for _, cp := range tc.present {
			response.Contains(row(cp))
		}
		for _, cp := range tc.absent {
			response.NotContains(row(cp))
		}
		// The chip bar rides along with the rows so it shows what is applied.
		response.Contains(`hx-swap-oob="outerHTML"`).Contains(`id="debt-aggregate-filters"`)
	}

	// The totals are summed over the filtered debts only: without the payable
	// side Alpha's net amount is the whole receivable.
	alphaOnly := "?f=counterparty:is:" + alpha.ID().String()
	suite.GET(DebtAggregateBasePath + alphaOnly).HTMX().Expect(t).Status(200).
		Contains("650.00")
	suite.GET(DebtAggregateBasePath + alphaOnly + "&f=type:is:RECEIVABLE").HTMX().Expect(t).Status(200).
		Contains("700.00").NotContains("650.00").NotContains("50.00")

	page := suite.GET(DebtAggregateBasePath + "?f=type:is:RECEIVABLE").Expect(t).Status(200)
	page.Contains(`x-data="filterBuilder"`).Contains(`value="type:is:RECEIVABLE"`).Contains("data-filter-bar").
		Contains(row(alpha)).NotContains(row(beta))

	// Back to a chip URL after a history-cache miss gets the whole page.
	suite.GET(DebtAggregateBasePath + "?f=type:is:RECEIVABLE").HTMX().HTMXHistoryRestore().Expect(t).Status(200).
		Contains("<html").Contains(`x-data="filterBuilder"`)
}
