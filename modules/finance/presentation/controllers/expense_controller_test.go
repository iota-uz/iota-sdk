package controllers_test

import (
	"context"
	"fmt"
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

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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
		Expect().
		Status(t, 200)

	html := response.HTML(t)
	require.GreaterOrEqual(t, len(html.Elements("//table//tbody//tr")), 2)

	response.Contains(t, "Test Category").
		Contains(t, "100.50").
		Contains(t, "200.75")
}

func TestExpenseController_List_HTMX_Request(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"HTMX Test Account",
		money.NewFromFloat(500.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"HTMX Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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
		Expect().
		Status(t, 200).
		Contains(t, "HTMX Test Category").
		Contains(t, "50.25")
}

func TestExpenseController_GetNew_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	_, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	createExpenseCategories(t, env.Ctx, category)

	response := suite.GET(ExpenseBasePath+"/new").
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//form[@hx-post]").Exists(t)
	html.Element("//input[@name='Amount']").Exists(t)
	html.Element("//select[@name='AccountID']").Exists(t)
	html.Element("//select[@name='CategoryID']").Exists(t)
	html.Element("//textarea[@name='Comment']").Exists(t)
	html.Element("//input[@name='Date']").Exists(t)
	html.Element("//input[@name='AccountingPeriod']").Exists(t)
}

func TestExpenseController_Create_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseCreate,
		permissions.ExpenseRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "150.75")
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "New test expense")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	suite.POST(ExpenseBasePath).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseBasePath)

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

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

	now := time.Now()
	formData := url.Values{}
	formData.Set("Amount", "0") // Invalid: must be greater than 0
	formData.Set("AccountID", createdAccount.ID().String())
	formData.Set("CategoryID", createdCategory.ID().String())
	formData.Set("Comment", "Test comment")
	formData.Set("Date", time.Time(shared.DateOnly(now)).Format(time.DateOnly))
	formData.Set("AccountingPeriod", time.Time(shared.DateOnly(now)).Format(time.DateOnly))

	response := suite.POST(ExpenseBasePath).
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
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

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Edit Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Edit Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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

	// Retrieve the created expense
	expenses, err := expenseService.GetAll(env.Ctx)
	require.NoError(t, err)
	require.NotEmpty(t, expenses)
	createdExpense := expenses[0]

	response := suite.GET(fmt.Sprintf("%s/%s", ExpenseBasePath, createdExpense.ID().String())).
		Expect().
		Status(t, 200)

	html := response.HTML(t)

	html.Element("//input[@name='Amount']").Exists(t)
	html.Element("//select[@name='AccountID']").Exists(t)
	html.Element("//select[@name='CategoryID']").Exists(t)
	html.Element("//textarea[@name='Comment']").Exists(t)
	require.Equal(t, "Edit test expense", html.Element("//textarea[@name='Comment']").Text())
}

func TestExpenseController_GetEdit_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.GET(fmt.Sprintf("%s/%s", ExpenseBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestExpenseController_Update_Success(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseUpdate,
		permissions.ExpenseRead,
		permissions.ExpenseCreate,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Update Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Update Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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

	suite.POST(fmt.Sprintf("%s/%s", ExpenseBasePath, expense1.ID().String())).
		WithForm(formData).
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseBasePath)

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

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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
		WithForm(formData).
		Expect().
		Status(t, 200)

	html := response.HTML(t)
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

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	expenseService := env.App.Service(services.ExpenseService{}).(*services.ExpenseService)
	moneyAccountService := env.App.Service(services.MoneyAccountService{}).(*services.MoneyAccountService)

	account := moneyAccountEntity.New(
		"Delete Test Account",
		money.NewFromFloat(1000.00, "USD"),
		moneyAccountEntity.WithTenantID(env.Tenant.ID),
	)

	createdAccount, err := moneyAccountService.Create(env.Ctx, account)
	require.NoError(t, err)

	category := expenseCategoryEntity.New(
		"Delete Test Category",
		expenseCategoryEntity.WithTenantID(env.Tenant.ID),
	)

	categories := createExpenseCategories(t, env.Ctx, category)
	createdCategory := categories[0]

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
		Expect().
		Status(t, 302).
		RedirectTo(t, ExpenseBasePath)

	_, err = expenseService.GetByID(env.Ctx, expense1.ID())
	require.Error(t, err)
}

func TestExpenseController_Delete_NotFound(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseDelete,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	nonExistentID := uuid.New()
	suite.DELETE(fmt.Sprintf("%s/%s", ExpenseBasePath, nonExistentID.String())).
		Expect().
		Status(t, 500)
}

func TestExpenseController_InvalidUUID(t *testing.T) {
	adminUser := testutils.MockUser(
		permissions.ExpenseRead,
	)

	suite := controllertest.New().
		WithModules(core.NewModule(), finance.NewModule()).
		WithUser(t, adminUser).
		Build(t)

	env := suite.Environment()
	createCurrencies(t, env.Ctx, &currency.USD)

	controller := controllers.NewExpensesController(env.App)
	suite.RegisterController(controller)

	suite.GET(ExpenseBasePath+"/invalid-uuid").
		Expect().
		Status(t, 404)
}

func createExpenseCategories(t *testing.T, ctx context.Context, categories ...expenseCategoryEntity.ExpenseCategory) []expenseCategoryEntity.ExpenseCategory {
	t.Helper()
	categoryRepo := persistence.NewExpenseCategoryRepository()
	results := make([]expenseCategoryEntity.ExpenseCategory, 0, len(categories))
	for _, cat := range categories {
		created, err := categoryRepo.Create(ctx, cat)
		require.NoError(t, err)
		results = append(results, created)
	}
	return results
}
