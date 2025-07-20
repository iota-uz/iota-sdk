package controllers_test

import (
	"fmt"
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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"HTMX Test Counterparty",
		counterparty.Supplier,
		counterparty.LegalEntity,
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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	response := suite.GET(DebtAggregateBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	// Should have table headers but no data rows
	require.Equal(t, 0, len(html.Elements("//table//tbody//tr")))
}

func TestDebtAggregateController_GetCounterpartyDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	suite.GET(DebtAggregateBasePath + "/invalid-uuid/drawer").
		Expect(t).
		Status(404)
}

func TestDebtAggregateController_Permission_Forbidden(t *testing.T) {
	t.Parallel()
	userWithoutPermission := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(userWithoutPermission)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
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

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtAggregateController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

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
		counterparty.LegalEntity,
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
