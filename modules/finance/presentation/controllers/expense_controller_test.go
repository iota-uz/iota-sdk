package controllers_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/finance"
	expenseAggregate "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense"
	expenseCategoryEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/expense_category"
	moneyAccountEntity "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/money_account"
	"github.com/iota-uz/iota-sdk/modules/finance/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/finance/permissions"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/testutils"
	"github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
	"github.com/stretchr/testify/require"
)

var (
	ExpenseBasePath = "/finance/expenses"
)

func TestExpenseController_List_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(100.50, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Test expense 1"),
	)

	expense2 := expenseAggregate.New(
		money.NewFromFloat(200.75, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Test expense 2"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)
	_, err = expenseService.Create(env.Ctx, expense2)
	require.NoError(t, err)

	response := suite.GET(ExpenseBasePath).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains("Test Category").
		Contains("100.50").
		Contains("200.75")
}

func TestExpenseController_List_HTMX_Request(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"HTMX Test Account",
		money.NewFromFloat(500.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"HTMX Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(50.25, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("HTMX Test Expense"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)

	suite.GET(ExpenseBasePath).
		HTMX().
		Expect(t).
		Status(200).
		Contains("HTMX Test Category").
		Contains("50.25")
}

func TestExpenseController_GetNew_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	_, err = categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	response := suite.GET(ExpenseBasePath + "/new").
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//form[@hx-post]").Exists()
	html.Element("//input[@name='Amount']").Exists()
	html.Element("//select[@name='AccountID']").Exists()
	html.Element("//select[@name='CategoryID']").Exists()
	html.Element("//textarea[@name='Comment']").Exists()
	html.Element("//input[@name='Date']").Exists()
	html.Element("//input[@name='AccountingPeriod']").Exists()
}

func TestExpenseController_Create_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCreate,
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "150.75")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "New test expense")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(ExpenseBasePath).Form(formData).Expect(t).Status(302).RedirectTo(ExpenseBasePath)

	expenses, err := expenseService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Len(t, expenses, 1)

	savedExpense := expenses[0]
	require.Equal(t, int64(15075), savedExpense.Amount().Amount())
	require.Equal(t, "New test expense", savedExpense.Comment())
}

func TestExpenseController_Create_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCreate,
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "0") // Invalid: must be greater than 0
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "Test comment")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	response := suite.POST(ExpenseBasePath).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	expenses, err := expenseService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.Empty(t, expenses)
}

func TestExpenseController_GetEdit_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
		permissions.ExpenseUpdate,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Edit Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Edit Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(250.00, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Edit test expense"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)

	expenses, err := expenseService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.NotEmpty(t, expenses)
	createdExpense := expenses[0]

	response := suite.GET(fmt.Sprintf("%s/%s", ExpenseBasePath, createdExpense.ID().String())).
		Expect(t).
		Status(200)

	html := response.HTML()

	html.Element("//input[@name='Amount']").Exists()
	html.Element("//select[@name='AccountID']").Exists()
	html.Element("//select[@name='CategoryID']").Exists()
	html.Element("//textarea[@name='Comment']").Exists()
	require.Equal(t, "Edit test expense", html.Element("//textarea[@name='Comment']").Text())
}

func TestExpenseController_GetEdit_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", ExpenseBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestExpenseController_Update_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseUpdate,
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Update Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Update Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Original expense"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "300.50")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "Updated expense comment")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(fmt.Sprintf("%s/%s", ExpenseBasePath, expense1.ID().String())).Form(formData).Expect(t).Status(http.StatusSeeOther)

	updatedExpense, err := expenseService.GetByID(env.Ctx, expense1.ID())
	require.NoError(t, err)

	require.Equal(t, int64(30050), updatedExpense.Amount().Amount())
	require.Equal(t, "Updated expense comment", updatedExpense.Comment())
}

func TestExpenseController_Update_ValidationError(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseUpdate,
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Test expense"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "0") // Invalid: must be greater than 0
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "Test comment")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	response := suite.POST(fmt.Sprintf("%s/%s", ExpenseBasePath, expense1.ID().String())).
		Form(formData).
		Expect(t).
		Status(200)

	html := response.HTML()
	require.NotEmpty(t, html.Elements("//small[@data-testid='field-error']"))

	unchangedExpense, err := expenseService.GetByID(env.Ctx, expense1.ID())
	require.NoError(t, err)
	require.Equal(t, "Test expense", unchangedExpense.Comment())
}

func TestExpenseController_Delete_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseDelete,
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Delete Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Delete Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(100.00, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Expense to Delete"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)

	existingExpense, err := expenseService.GetByID(env.Ctx, expense1.ID())
	require.NoError(t, err)
	require.Equal(t, "Expense to Delete", existingExpense.Comment())

	suite.DELETE(fmt.Sprintf("%s/%s", ExpenseBasePath, expense1.ID().String())).
		Expect(t).
		Status(302).
		RedirectTo(ExpenseBasePath)

	_, err = expenseService.GetByID(env.Ctx, expense1.ID())
	require.Error(t, err)
}

func TestExpenseController_Delete_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseDelete,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", ExpenseBasePath, nonExistentID.String())).
		Expect(t).
		Status(500)
}

func TestExpenseController_InvalidUUID(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	suite.GET(ExpenseBasePath + "/invalid-uuid").
		Expect(t).
		Status(404)
}

func TestExpenseController_Export_Excel_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Export Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	categoryRepo := persistence.NewExpenseCategoryRepository()
	category := expenseCategoryEntity.New(
		"Export Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createdCategory, err := categoryRepo.Create(env.Ctx, category)
	require.NoError(t, err)

	expense1 := expenseAggregate.New(
		money.NewFromFloat(100.50, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Export test expense 1"),
	)

	expense2 := expenseAggregate.New(
		money.NewFromFloat(200.75, "USD"),
		createdAccount,
		createdCategory,
		time.Now(),
		expenseAggregate.WithTenantID(env.Tenant.ID),
		expenseAggregate.WithComment("Export test expense 2"),
	)

	_, err = expenseService.Create(env.Ctx, expense1)
	require.NoError(t, err)
	_, err = expenseService.Create(env.Ctx, expense2)
	require.NoError(t, err)

	response := suite.POST(ExpenseBasePath + "/export?format=excel").
		Expect(t)

	// Accept either 302 or 303 status codes (both are valid for redirects)
	rawResponse := response.Raw()
	defer func() { _ = rawResponse.Body.Close() }()
	statusCode := rawResponse.StatusCode
	require.True(t, statusCode == 302 || statusCode == 303, "Expected 302 or 303, got %d", statusCode)

	redirectLocation := response.Header("Location")
	require.NotEmpty(t, redirectLocation)
	require.Contains(t, redirectLocation, ".xlsx") // Check for Excel file extension
}

func TestExpenseController_Export_InvalidFormat(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	suite.POST(ExpenseBasePath + "/export?format=invalid-format").
		Expect(t).
		Status(400).
		Contains("Invalid export format")
}

func TestExpenseController_Export_MissingFormat(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(adminUser)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	suite.POST(ExpenseBasePath + "/export").
		Expect(t).
		Status(400).
		Contains("Invalid export format")
}

func TestExpenseController_Export_Forbidden(t *testing.T) {
	userWithoutPermission := testutils.MockUser()

	suite := controllertest.New(t, core.NewModule(), finance.NewModule()).
		AsUser(userWithoutPermission)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.Register(controller)

	suite.POST(ExpenseBasePath + "/export?format=excel").
		Expect(t).
		Status(403).
		Contains("forbidden")
}
