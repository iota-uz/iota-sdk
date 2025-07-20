package controllers_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	debtAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/debt"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/counterparty"
	transactionEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/entities/transaction"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/stretchr/testify/require"
)

var (
	DebtBasePath = "/finance/debts"
)

func TestDebtController_List_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
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
		money.NewFromFloat(250.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Test receivable debt 1"),
	)

	debt2 := debtAggregate.New(
		debtAggregate.DebtTypePayable,
		money.NewFromFloat(175.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Test payable debt 2"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)
	_, err = debtService.Create(env.Ctx, debt2)
	require.NoError(t, err)

	response := suite.GET(DebtBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Debt Counterparty").
		Contains("250.00").
		Contains("175.00")
}

func TestDebtController_List_HTMX_Request(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
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
		money.NewFromFloat(300.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("HTMX test debt"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	suite.GET(DebtBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Counterparty").
		Contains("300.00")
}

func TestDebtController_GetEditDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
		permissions.DebtUpdate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Edit Drawer Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(400.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Edit test debt"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	response := suite.GET(fmt.Sprintf("%s/%s/drawer", DebtBasePath, createdDebt.ID().String())).
		Expect(t).
		Status(200)

	response.Contains("Edit Drawer Test Counterparty").
		Contains("Edit test debt").
		Contains("400.00")

	html := response.HTML()
	html.Element("//form[@hx-post]").Exists()
}

func TestDebtController_GetEditDrawer_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s/drawer", DebtBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestDebtController_GetNewDrawer_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty for dropdown
	counterparty1 := counterparty.New(
		"Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	_, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	response := suite.GET(DebtBasePath + "/new/drawer").
		Expect(t).
		Status(200)

	html := response.HTML()
	html.Element("//form[@hx-post]").Exists()
	html.Element("//select[@name='CounterpartyID']").Exists()
	html.Element("//input[@name='Amount']").Exists()
	html.Element("//select[@name='Type']").Exists()
	html.Element("//textarea[@name='Description']").Exists()
}

func TestDebtController_Create_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtCreate,
		permissions.DebtRead,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Create Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Amount", "500.75")
	formData.Set("Type", "RECEIVABLE")
	formData.Set("Description", "New test debt")
	formData.Set("DueDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(DebtBasePath).
		Form(formData).
		Header("HX-Request", "true").
		Header("HX-Target", "debt-create-drawer").
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	debts, err := debtService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, debts, 1)

	savedDebt := debts[0]
	require.Equal(t, int64(50075), savedDebt.OriginalAmount().Amount())
	require.Equal(t, "New test debt", savedDebt.Description())
	require.Equal(t, debtAggregate.DebtTypeReceivable, savedDebt.Type())
}

func TestDebtController_Create_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtCreate,
		permissions.DebtRead,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Validation Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Amount", "0") // Invalid: must be greater than 0
	formData.Set("Type", "RECEIVABLE")
	formData.Set("Description", "Test debt")
	formData.Set("DueDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	response := suite.POST(DebtBasePath).
		Form(formData).
		Header("HX-Request", "true").
		Header("HX-Target", "debt-create-drawer").
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	debts, err := debtService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, debts)
}

func TestDebtController_Update_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtUpdate,
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Update Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(200.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Original debt description"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Amount", "350.25")
	formData.Set("Type", "PAYABLE")
	formData.Set("Description", "Updated debt description")
	formData.Set("DueDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(fmt.Sprintf("%s/%s", DebtBasePath, createdDebt.ID().String())).
		Form(formData).
		Header("HX-Request", "true").
		Header("HX-Target", "debt-edit-drawer").
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	updatedDebt, err := debtService.GetByID(env.Ctx, createdDebt.ID())
	require.NoError(t, err)

	require.Equal(t, int64(35025), updatedDebt.OriginalAmount().Amount())
	require.Equal(t, "Updated debt description", updatedDebt.Description())
	require.Equal(t, debtAggregate.DebtTypePayable, updatedDebt.Type())
}

func TestDebtController_Update_ValidationError(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtUpdate,
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Update Validation Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(100.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Original debt"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("CounterpartyID", createdCounterparty.ID().String())
	formData.Set("Amount", "0") // Invalid: must be greater than 0
	formData.Set("Type", "RECEIVABLE")
	formData.Set("Description", "Test description")
	formData.Set("DueDate", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	response := suite.POST(fmt.Sprintf("%s/%s", DebtBasePath, createdDebt.ID().String())).
		Form(formData).
		Header("HX-Request", "true").
		Header("HX-Target", "debt-edit-drawer").
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedDebt, err := debtService.GetByID(env.Ctx, createdDebt.ID())
	require.NoError(t, err)
	require.Equal(t, "Original debt", unchangedDebt.Description())
}

func TestDebtController_Settle_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtUpdate,
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)
	transactionService := env.App.Service(services.TransactionService{}).(*services.TransactionService)

	// Create test money account
	account := moneyAccountEntity.New(
		"Test Settlement Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Settlement Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(500.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Settlement test debt"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	// Create settlement transaction
	transaction := transactionEntity.New(
		money.NewFromFloat(300.00, "USD"),
		transactionEntity.Deposit,
		transactionEntity.WithTenantID(env.Tenant.ID),
		transactionEntity.WithOriginAccountID(createdAccount.ID()),
		transactionEntity.WithComment("Settlement payment"),
		transactionEntity.WithTransactionDate(time.Now()),
	)

	createdTransaction, err := transactionService.Create(env.Ctx, transaction)
	require.NoError(t, err)

	formData := url.Values{}
	formData.Set("SettlementAmount", "300.00")
	formData.Set("TransactionID", createdTransaction.ID().String())

	suite.POST(fmt.Sprintf("%s/%s/settle", DebtBasePath, createdDebt.ID().String())).
		Form(formData).
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	settledDebt, err := debtService.GetByID(env.Ctx, createdDebt.ID())
	require.NoError(t, err)

	require.Equal(t, int64(20000), settledDebt.OutstandingAmount().Amount()) // 500 - 300 = 200
	require.Equal(t, debtAggregate.DebtStatusPartial, settledDebt.Status())
}

func TestDebtController_WriteOff_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtUpdate,
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"WriteOff Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(250.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("WriteOff test debt"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	suite.POST(fmt.Sprintf("%s/%s/write-off", DebtBasePath, createdDebt.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	writtenOffDebt, err := debtService.GetByID(env.Ctx, createdDebt.ID())
	require.NoError(t, err)

	require.Equal(t, debtAggregate.DebtStatusWrittenOff, writtenOffDebt.Status())
}

func TestDebtController_Delete_Success(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtDelete,
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Delete Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debt
	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(150.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Delete test debt"),
	)

	createdDebt, err := debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	// Verify debt exists
	existingDebt, err := debtService.GetByID(env.Ctx, createdDebt.ID())
	require.NoError(t, err)
	require.Equal(t, "Delete test debt", existingDebt.Description())

	suite.DELETE(fmt.Sprintf("%s/%s", DebtBasePath, createdDebt.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(DebtBasePath)

	// Verify debt is deleted
	_, err = debtService.GetByID(env.Ctx, createdDebt.ID())
	require.Error(t, err)
}

func TestDebtController_Delete_NotFound(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtDelete,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", DebtBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestDebtController_InvalidUUID(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	suite.GET(DebtBasePath + "/invalid-uuid/drawer").
		Expect(t).
		Status(404)
}

func TestDebtController_Permission_Forbidden(t *testing.T) {
	t.Parallel()
	userWithoutPermission := itf.User()

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(userWithoutPermission)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	suite.GET(DebtBasePath).
		Expect(t).
		Status(403).
		Contains("forbidden")
}

func TestDebtController_List_WithFilters(t *testing.T) {
	t.Parallel()
	adminUser := itf.User(
		permissions.DebtRead,
		permissions.DebtCreate,
	)

	suite := itf.HTTP(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewDebtsController(env.App)
	suite.Register(controller)

	debtService := env.App.Service(services.DebtService{}).(*services.DebtService)
	counterpartyService := env.App.Service(services.CounterpartyService{}).(*services.CounterpartyService)

	// Create test counterparty
	counterparty1 := counterparty.New(
		"Filter Test Counterparty",
		counterparty.Customer,
		counterparty.Individual,
		counterparty.WithTenantID(env.Tenant.ID),
	)

	createdCounterparty, err := counterpartyService.Create(env.Ctx, counterparty1)
	require.NoError(t, err)

	// Create test debts with different dates
	yesterday := time.Now().AddDate(0, 0, -1)
	// tomorrow := time.Now().AddDate(0, 0, 1) // Not needed for filter test

	debt1 := debtAggregate.New(
		debtAggregate.DebtTypeReceivable,
		money.NewFromFloat(100.00, "USD"),
		debtAggregate.WithTenantID(env.Tenant.ID),
		debtAggregate.WithCounterpartyID(createdCounterparty.ID()),
		debtAggregate.WithDescription("Yesterday debt"),
	)

	_, err = debtService.Create(env.Ctx, debt1)
	require.NoError(t, err)

	// Test with date filter
	fromDate := yesterday.Format("2006-01-02")
	toDate := time.Now().Format("2006-01-02")

	response := suite.GET(DebtBasePath + "?CreatedAt.From=" + fromDate + "&CreatedAt.To=" + toDate).
		Expect(t).
		Status(200)

	// Should contain yesterday's debt
	response.Contains("Yesterday debt")
}
